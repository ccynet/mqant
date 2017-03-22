// Copyright 2014 mqant Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package mqrpc

import (
	"github.com/liangdas/mqant/conf"
	"github.com/liangdas/mqant/log"
	"github.com/liangdas/mqant/utils/uuid"
	"time"
	"fmt"
	"reflect"
	"encoding/base64"
)

type RPCClient struct {
	remote_client *AMQPClient
	local_client  *LocalClient
}

func NewRPCClient() (*RPCClient, error) {
	rpc_client := new(RPCClient)
	return rpc_client, nil
}

func (c *RPCClient) NewRemoteClient(info *conf.Rabbitmq) (err error) {
	//创建本地连接
	if info != nil && c.remote_client == nil {
		c.remote_client, err = NewAMQPClient(info)
		if err != nil {
			log.Error("Dial: %s", err)
		}
	}
	return
}

func (c *RPCClient) NewLocalClient(server *RPCServer) (err error) {
	//创建本地连接
	if server != nil && server.local_server != nil && c.local_client == nil {
		c.local_client, err = NewLocalClient(server.local_server)
		if err != nil {
			log.Error("Dial: %s", err)
		}
	}
	return
}

func (c *RPCClient) Done() (err error) {
	if c.remote_client != nil {
		err = c.remote_client.Done()
	}
	if c.local_client != nil {
		err = c.local_client.Done()
	}
	return
}

/**
消息请求 需要回复
*/
func (c *RPCClient) Call(_func string, params ...interface{}) (interface{}, string) {
	var ArgsType []string=nil
	var args []interface{}=make([]interface{}, len(params))
	if len(params) > 0 {
		//prepare in paras
		ArgsType = make([]string, len(params))
		for k, param := range params {
			switch v2:=param.(type) {
			case string:
				ArgsType[k] = STRING
				args[k]=params[k]
			case bool:
				ArgsType[k] = BOOL
				args[k]=params[k]
			case int:
				ArgsType[k] = INT
				args[k]=params[k]
			case int64:
				ArgsType[k] = LONG
				args[k]=params[k]
			case float32:
				ArgsType[k] = FLOAT
				args[k]=params[k]
			case float64:
				ArgsType[k] = DOUBLE
				args[k]=params[k]
			case []byte:
				//这里要把[]byte转base64
				ArgsType[k] = BYTES
				uEnc := base64.URLEncoding.EncodeToString(v2)
				args[k]=uEnc
			case map[string]interface{}:
				ArgsType[k] = MAP
				args[k]=params[k]
			case map[string]string:
				ArgsType[k] = MAP
				args[k]=params[k]
			default:
				return nil, fmt.Sprintf( "args[%d] [%s] Types not allowed",k,reflect.TypeOf(param))
			}

		}
	}
	var correlation_id = uuid.Rand().Hex()
	callInfo := &CallInfo{
		Fn:      _func,
		Args:    args,
		ArgsType:ArgsType,
		Reply:   true,                                                                                      //客户端是否需要结果
		Expired: (time.Now().UTC().Add(time.Second * time.Duration(conf.RpcExpired)).UnixNano()) / 1000000, //超时日期 unix 时间戳 单位/毫秒 要求服务端与客户端时间精准同步
		Cid:     correlation_id,
	}

	callback := make(chan ResultInfo, 1)
	var err error

	//优先使用本地rpc
	if c.local_client != nil {
		err = c.local_client.Call(*callInfo, callback)
	} else {
		if c.remote_client != nil {
			err = c.remote_client.Call(*callInfo, callback)
		} else {
			return nil, "rpc service connection failed"
		}
	}

	if err != nil {
		return nil, err.Error()
	}

	resultInfo, ok := <-callback
	if !ok {
		return nil, "client closed"
	}
	return resultInfo.Result, resultInfo.Error
}

/**
消息请求 不需要回复
*/
func (c *RPCClient) CallNR(_func string, params ...interface{}) (err error) {
	var ArgsType []string=nil
	var args []interface{}=make([]interface{}, len(params))
	if len(params) > 0 {
		//prepare in paras
		ArgsType = make([]string, len(params))
		for k, param := range params {
			switch v2:=param.(type) {
			case string:
				ArgsType[k] = STRING
				args[k]=params[k]
			case bool:
				ArgsType[k] = BOOL
				args[k]=params[k]
			case int:
				ArgsType[k] = INT
				args[k]=params[k]
			case int64:
				ArgsType[k] = LONG
				args[k]=params[k]
			case float32:
				ArgsType[k] = FLOAT
				args[k]=params[k]
			case float64:
				ArgsType[k] = DOUBLE
				args[k]=params[k]
			case []byte:
				//这里要把[]byte转base64
				ArgsType[k] = BYTES
				uEnc := base64.URLEncoding.EncodeToString(v2)
				args[k]=uEnc
			case map[string]interface{}:
				ArgsType[k] = MAP
				args[k]=params[k]
			case map[string]string:
				ArgsType[k] = MAP
				args[k]=params[k]
			default:
				return fmt.Errorf( "args[%d] [%s] Types not allowed",k,reflect.TypeOf(param))
			}

		}
	}
	var correlation_id = uuid.Rand().Hex()
	callInfo := &CallInfo{
		Fn:      _func,
		Args:    args,
		Reply:   false,                                                                                     //客户端是否需要结果
		Expired: (time.Now().UTC().Add(time.Second * time.Duration(conf.RpcExpired)).UnixNano()) / 1000000, //超时日期 unix 时间戳 单位/毫秒 要求服务端与客户端时间精准同步
		Cid:     correlation_id,
	}

	//优先使用本地rpc
	if c.local_client != nil {
		err = c.local_client.CallNR(*callInfo)
	} else {
		if c.remote_client != nil {
			err = c.remote_client.CallNR(*callInfo)
		} else {
			return fmt.Errorf("rpc service connection failed")
		}
	}

	if err != nil {
		return err
	}
	return nil
}
