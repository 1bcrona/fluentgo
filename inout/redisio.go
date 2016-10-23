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
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/garyburd/redigo/redis"
	"github.com/ocdogan/fluentgo/lib"
	"github.com/ocdogan/fluentgo/log"
)

type redisIO struct {
	id       lib.UUID
	db       int
	command  string
	server   string
	password string
	channel  string
	poolName string
	connFunc func(redis.Conn) error
	conn     redis.Conn
	logger   log.Logger
}

func newRedisIO(logger log.Logger, params map[string]interface{}) *redisIO {
	if params == nil {
		return nil
	}

	id, err := lib.NewUUID()
	if err != nil {
		return nil
	}

	var (
		ok       bool
		db       int
		f        float64
		s        string
		poolName string
		command  string
		server   string
		channel  string
		password string
	)

	s, ok = params["poolName"].(string)
	if ok {
		poolName = strings.TrimSpace(s)
	}

	s, ok = params["server"].(string)
	if ok {
		server = strings.TrimSpace(s)
	}
	if server == "" {
		return nil
	}

	s, ok = params["channel"].(string)
	if ok {
		channel = strings.TrimSpace(s)
	}
	if channel == "" {
		return nil
	}

	s, ok = params["password"].(string)
	if ok {
		password = strings.TrimSpace(s)
	}

	s, ok = params["command"].(string)
	if ok {
		command = strings.ToUpper(strings.TrimSpace(s))
	}

	f, ok = params["db"].(float64)
	if ok {
		db = lib.MinInt(15, lib.MaxInt(0, int(f)))
	}

	rio := &redisIO{
		id:       *id,
		db:       db,
		command:  command,
		server:   server,
		poolName: poolName,
		password: password,
		channel:  channel,
		logger:   logger,
	}

	return rio
}

func (rio *redisIO) ID() lib.UUID {
	return rio.id
}

func (rio *redisIO) funcAfterClose() {
	defer recover()

	conn := rio.conn
	if conn != nil {
		rio.conn = nil
		conn.Close()
	}
}

func (rio *redisIO) tryToCloseConn(conn redis.Conn) error {
	var closeErr error
	if conn != nil {
		defer func() {
			err := recover()
			if closeErr == nil {
				closeErr, _ = err.(error)
			}
		}()
		closeErr = conn.Close()
	}
	return closeErr
}

func (rio *redisIO) selectDb(conn redis.Conn) error {
	var connErr error
	defer func() {
		err := recover()
		if err != nil && connErr == nil {
			defer recover()
			connErr, _ = err.(error)
			rio.tryToCloseConn(conn)
		}
	}()

	_, connErr = conn.Do("SELECT", rio.db)
	return connErr
}

func (rio *redisIO) runConnFunc(conn redis.Conn) error {
	var funcErr error

	cfn := rio.connFunc
	if cfn != nil {
		defer func() {
			err := recover()
			if err != nil && funcErr == nil {
				defer recover()
				funcErr, _ = err.(error)
				conn.Close()
			}
		}()

		funcErr = cfn(conn)
	}
	return funcErr
}

func (rio *redisIO) ping(conn redis.Conn) error {
	if conn == nil {
		return errors.New("No Redis connection")
	}

	var subsErr error
	defer func() {
		err, _ := recover().(error)
		if subsErr == nil {
			subsErr = err
		}
	}()

	var rep interface{}
	rep, subsErr = conn.Do("PING")
	if subsErr == nil {
		s, ok := rep.(string)
		if !ok || strings.ToUpper(s) != "PONG" {
			subsErr = fmt.Errorf("Unable to ping Redis '%s'", conn)
		}
	}
	return subsErr
}

func (rio *redisIO) Connect() {
	defer func() {
		if err := recover(); err != nil {
			if rio.logger != nil {
				rio.logger.Panic(err)
			}
		}
	}()

	conn := rio.conn
	hasConn := conn != nil && !reflect.ValueOf(conn).IsNil()

	var connErr error

	if hasConn {
		connErr = conn.Err()
		if connErr != nil {
			rio.conn = nil
		}
	}

	if connErr != nil || !hasConn {
		if hasConn {
			rio.tryToCloseConn(conn)
		}

		connErr = nil
		conn = getRedisConnection(rio.poolName, rio.server, rio.password)

		if conn == nil {
			l := rio.logger
			if l != nil {
				l.Printf("Cannot connect to REDIS: %s, %s\n", rio.poolName, rio.server)
			}
		} else {
			connErr = rio.selectDb(conn)
			if connErr != nil && rio.logger != nil {
				rio.logger.Println(connErr)
			}

			connErr = rio.ping(conn)
			if connErr == nil {
				connErr = rio.runConnFunc(conn)
			}
		}

		if connErr == nil {
			rio.conn = conn
		} else {
			rio.conn = nil
			rio.tryToCloseConn(conn)
		}
	}
}