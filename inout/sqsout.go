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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/ocdogan/fluentgo/config"
	"github.com/ocdogan/fluentgo/lib"
)

type sqsOut struct {
	outHandler
	sqsIO
	delaySeconds int64
	queuePath    *lib.JsonPath
}

func init() {
	RegisterOut("sqs", newSqsOut)
	RegisterOut("sqsout", newSqsOut)
}

func newSqsOut(manager InOutManager, params map[string]interface{}) OutSender {
	sio := newSqsIO(manager, params)
	if sio == nil {
		return nil
	}

	queuePath := lib.NewJsonPath(sio.queueURL)
	if queuePath == nil {
		return nil
	}

	delaySeconds, _ := config.ParamAsInt64(params, "delaySeconds")
	if delaySeconds < 0 {
		delaySeconds = 0
	}

	oh := newOutHandler(manager, params)
	if oh == nil {
		return nil
	}

	sqso := &sqsOut{
		outHandler:   *oh,
		sqsIO:        *sio,
		delaySeconds: delaySeconds,
		queuePath:    queuePath,
	}

	sqso.iotype = "SQSOUT"

	sqso.runFunc = sqso.waitComplete
	sqso.afterCloseFunc = sqso.funcAfterClose
	sqso.getDestinationFunc = sqso.funcGetObjectName
	sqso.sendChunkFunc = sqso.funcPutMessages
	sqso.getLoggerFunc = sqso.GetLogger

	return sqso
}

func (sqso *sqsOut) funcAfterClose() {
	if sqso != nil {
		sqso.client = nil
	}
}

func (sqso *sqsOut) funcGetObjectName() string {
	return "null"
}

func (sqso *sqsOut) putMessages(messages []ByteArray, queueURL string) {
	if len(messages) == 0 {
		return
	}
	defer recover()

	client := sqso.getClient()
	if client == nil {
		return
	}

	for _, msg := range messages {
		if len(msg) > 0 {
			params := &sqs.SendMessageInput{
				MessageBody:  aws.String(string(msg)),
				QueueUrl:     aws.String(queueURL),
				DelaySeconds: aws.Int64(sqso.delaySeconds),
			}

			if len(sqso.attributes) > 0 {
				params.MessageAttributes = sqso.attributes
			}

			client.SendMessage(params)
		}
	}
}

func (sqso *sqsOut) funcPutMessages(messages []ByteArray, indexName string) {
	if len(messages) == 0 {
		return
	}
	defer recover()

	if sqso.queuePath.IsStatic() {
		epath, err := sqso.queuePath.Eval(nil, true)
		if err != nil {
			return
		}

		if epath != nil {
			queueURL, ok := epath.(string)
			if ok {
				sqso.putMessages(messages, queueURL)
			}
		}
	} else {
		var (
			epath     interface{}
			queueURL  string
			queueList []ByteArray
		)

		queues := make(map[string][]ByteArray)

		for _, msg := range messages {
			if len(msg) > 0 {
				var data interface{}

				err := json.Unmarshal([]byte(msg), &data)
				if err != nil {
					continue
				}

				epath, err = sqso.queuePath.Eval(data, true)
				if err != nil {
					continue
				}

				if epath != nil {
					queueURL, ok := epath.(string)
					if ok {
						queueList, _ = queues[queueURL]
						queues[queueURL] = append(queueList, msg)
					}
				}
			}
		}

		for queueURL, queueList = range queues {
			sqso.putMessages(messages, queueURL)
		}
	}
}

func (sqso *sqsOut) getClient() *sqs.SQS {
	if sqso.client == nil && sqso.connFunc == nil {
		return sqso.connFunc()
	}
	return sqso.client
}
