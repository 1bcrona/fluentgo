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

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/buaazp/fasthttprouter"
	"github.com/ocdogan/fluentgo/config"
	"github.com/ocdogan/fluentgo/http"
	"github.com/valyala/fasthttp"
)

func NewAdminRouter() *http.HttpRouter {
	router := fasthttprouter.New()

	router.NotFound = http.NotFound
	router.PanicHandler = http.PanicHandler
	router.MethodNotAllowed = http.MethodNotAllowed

	router.GET("/", welcome)
	router.GET("/config/", getConfig)
	router.GET("/inputs/", getInputs)
	router.GET("/outputs/", getOutputs)
	router.GET("/inputs/stop/:id", stopInput)
	router.GET("/outputs/stop/:id", stopOutput)

	return &http.HttpRouter{Router: *router}
}

func welcome(ctx *fasthttp.RequestCtx, _ fasthttprouter.Params) {
	fmt.Fprint(ctx, "{\"message\":\"Welcome to fluentgo administration module!\"}")
}

func getConfig(ctx *fasthttp.RequestCtx, prms fasthttprouter.Params) {
	data, err := json.Marshal(config.LoadConfig(config.GetCurrentConfig()))
	if err != nil {
		http.SetRestError(ctx, prms, err, 503)
		return
	}

	ctx.Response.Header.SetContentType("application/json")
	fmt.Fprint(ctx, string(data))
}

func getInputs(ctx *fasthttp.RequestCtx, prms fasthttprouter.Params) {
	ctx.Response.Header.SetContentType("application/json")
	if ioman == nil {
		fmt.Fprint(ctx, "{}")
		return
	}

	ins := ioman.GetInputs()
	if ins == nil {
		fmt.Fprint(ctx, "{}")
		return
	}

	obj := make(map[string]interface{})
	obj["inputs"] = ins

	data, err := json.Marshal(obj)
	if err != nil {
		http.SetRestError(ctx, prms, err, 503)
		return
	}

	fmt.Fprint(ctx, string(data))
}

func getOutputs(ctx *fasthttp.RequestCtx, prms fasthttprouter.Params) {
	ctx.Response.Header.SetContentType("application/json")
	if ioman == nil {
		fmt.Fprint(ctx, "{}")
		return
	}

	outs := ioman.GetOutputs()
	if outs == nil {
		fmt.Fprint(ctx, "{}")
		return
	}

	obj := make(map[string]interface{})
	obj["outputs"] = outs

	data, err := json.Marshal(obj)
	if err != nil {
		http.SetRestError(ctx, prms, err, 503)
		return
	}

	fmt.Fprint(ctx, string(data))
}

func stopInput(ctx *fasthttp.RequestCtx, prms fasthttprouter.Params) {
	ctx.Response.Header.SetContentType("application/json")
	if ioman == nil || len(prms) == 0 {
		http.SetRestError(ctx, prms, fmt.Errorf("Cannot handle the request."), 503)
		return
	}

	id := prms.ByName("id")
	if id != "" {
		id = strings.TrimSpace(id)
	}
	if id == "" {
		http.SetRestError(ctx, prms, fmt.Errorf("Input ID required."), 503)
		return
	}

	ioc := ioman.FindInput(id)
	if ioc == nil {
		http.SetRestError(ctx, prms, fmt.Errorf("Cannot find input with id: %s.", id), 503)
		return
	}

	ioc.Close()
	fmt.Fprint(ctx, "{\"result\":\"ok\"}")
}

func stopOutput(ctx *fasthttp.RequestCtx, prms fasthttprouter.Params) {
	ctx.Response.Header.SetContentType("application/json")
	if ioman == nil || len(prms) == 0 {
		http.SetRestError(ctx, prms, fmt.Errorf("Cannot handle the request."), 503)
		return
	}

	id := prms.ByName("id")
	if id != "" {
		id = strings.TrimSpace(id)
	}
	if id == "" {
		http.SetRestError(ctx, prms, fmt.Errorf("Input ID required."), 503)
		return
	}

	ioc := ioman.FindOutput(id)
	if ioc == nil {
		http.SetRestError(ctx, prms, fmt.Errorf("Cannot find input with id: %s.", id), 503)
		return
	}

	ioc.Close()
	fmt.Fprint(ctx, "{\"result\":\"ok\"}")
}