//	The MIT License (MIT)
//
//	Copyright (c) 2016, Cagatay Dogan
//
//	Permission is hereby granted, free of charge, to any person obtaining a copy
//	of this software and associated documentation files (the "Software"), to deal
//	in the Software without restriction, including without limitation the rights
//	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//	copies of the Software, and to permit persons to whom the Software is
//	furnished to do so, subject to the following conditions:
//
//		The above copyright notice and this permission notice shall be included in
//		all copies or substantial portions of the Software.
//
//		THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//		IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//		FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//		AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//		LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//		OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
//		THE SOFTWARE.

package inout

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ocdogan/fluentgo/config"
	"github.com/ocdogan/fluentgo/lib"
	"github.com/ocdogan/fluentgo/log"
	"gopkg.in/mgo.v2"
)

type mongOut struct {
	sync.Mutex
	outHandler
	servers        string
	db             string
	dialTimeout    time.Duration
	collectionPath *lib.JsonPath
	lg             log.Logger
	session        *mgo.Session
}

func init() {
	RegisterOut("mongo", newMongOut)
	RegisterOut("mongout", newMongOut)
	RegisterOut("mongoout", newMongOut)
}

func newMongOut(manager InOutManager, params map[string]interface{}) OutSender {
	oh := newOutHandler(manager, params)
	if oh == nil {
		return nil
	}

	servers, ok := config.ParamAsString(params, "servers")
	if !ok || servers == "" {
		return nil
	}

	db, ok := config.ParamAsString(params, "db")
	if !ok || db == "" {
		return nil
	}

	collection, ok := config.ParamAsString(params, "collection")
	if !ok || collection == "" {
		return nil
	}

	dialTimeout, ok := config.ParamAsDurationWithLimit(params, "dialTimeoutMSec", 0, 60000)
	if ok {
		dialTimeout *= time.Millisecond
	}

	collectionPath := lib.NewJsonPath(collection)
	if collectionPath == nil {
		return nil
	}

	mo := &mongOut{
		servers:        servers,
		db:             db,
		dialTimeout:    dialTimeout,
		collectionPath: collectionPath,
		lg:             manager.GetLogger(),
	}

	mo.iotype = "MONGOUT"

	mo.runFunc = mo.waitComplete
	mo.afterCloseFunc = mo.funcAfterClose
	mo.getDestinationFunc = mo.funcGetObjectName
	mo.sendChunkFunc = mo.funcPutMessages

	return mo
}

func (mo *mongOut) funcAfterClose() {
	if mo.session != nil {
		defer recover()

		mo.Lock()
		defer mo.Unlock()

		if mo.session != nil {
			session := mo.session
			mo.session = nil

			session.Close()
		}
	}
}

func (mo *mongOut) funcGetObjectName() string {
	return "null"
}

func (mo *mongOut) putMessages(messages []string, collection string) {
	if len(messages) == 0 {
		return
	}
	defer recover()

	err := mo.Connect()
	if err != nil || mo.session == nil {
		return
	}

	doSend := false
	bulk := mo.session.DB(mo.db).C(collection).Bulk()

	for _, msg := range messages {
		if msg != "" {
			doSend = true
			bulk.Insert(msg)
		}
	}

	if doSend {
		_, err := bulk.Run()
		if err != nil {
			l := mo.GetLogger()
			if l != nil {
				l.Printf("Cannot send MONGOUT message to %s:%s: %s", mo.db, collection, err)
			}
		}
	}
}

func (mo *mongOut) funcPutMessages(messages []string, collection string) {
	if len(messages) == 0 {
		return
	}
	defer recover()

	if mo.collectionPath.IsStatic() {
		collection, _, err := mo.collectionPath.Eval(nil, true)
		if err != nil {
			return
		}

		mo.putMessages(messages, collection)
	} else {
		var (
			collection     string
			collectionList []string
		)

		collections := make(map[string][]string)

		for _, msg := range messages {
			if msg != "" {
				var data interface{}

				err := json.Unmarshal([]byte(msg), &data)
				if err != nil {
					continue
				}

				collection, _, err = mo.collectionPath.Eval(data, true)
				if err != nil {
					continue
				}

				collectionList, _ = collections[collection]
				collections[collection] = append(collectionList, msg)
			}
		}

		for collection, collectionList = range collections {
			mo.putMessages(messages, collection)
		}
	}
}

func (mo *mongOut) Connect() error {
	if mo.session == nil {
		mo.Lock()
		defer mo.Unlock()

		if mo.session == nil {
			var (
				session *mgo.Session
				err     error
			)

			if mo.dialTimeout == 0 {
				session, err = mgo.Dial(mo.servers)
			} else {
				session, err = mgo.DialWithTimeout(mo.servers, mo.dialTimeout)
			}

			if err != nil {
				if mo.lg != nil {
					mo.lg.Printf("Failed to create MONGOUT session: %s\n", err)
				}
				return err
			}

			// Optional. Switch the session to a monotonic behavior.
			session.SetMode(mgo.Monotonic, true)

			mo.session = session
		}
	}
	return nil
}