// Package main 演示在 HTTP 服务里使用 errorx：
// 业务层只管 return error，出口统一翻译成 {code, msg}。
//
// 运行：go run ./example/http
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/yuninks/errorx"
	"github.com/yuninks/langx"
)

var (
	ErrParamInvalid = errorx.New("param_invalid", 10002, "参数错误: #field#")
	ErrUserNotFound = errorx.New("user_not_found", 10001, "用户不存在")
	ErrDB           = errorx.New("db_error", 50001, "数据库繁忙")
)

func init() {
	langx.AppendTrans("en", map[string]string{
		"param_invalid":  "invalid parameter: #field#",
		"user_not_found": "user not found",
		"db_error":       "database is busy",
	})
}

// Response 是对外的统一响应体
type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// 业务层：只返回 error，完全不关心 HTTP 与语言
func loadUser(ctx context.Context, id string) (string, error) {
	switch id {
	case "":
		return "", ErrParamInvalid.ErrKV("field", "id")
	case "404":
		return "", ErrUserNotFound.Err()
	case "500":
		return "", ErrDB.Wrap(errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"))
	default:
		return "用户-" + id, nil
	}
}

// 出口层：把 error 统一翻译成响应
func userHandler(w http.ResponseWriter, r *http.Request) {
	ctx := withLang(r)
	name, err := loadUser(ctx, r.URL.Query().Get("id"))
	if err != nil {
		writeJSON(w, toResponse(ctx, err))
		return
	}
	writeJSON(w, Response{Code: errorx.Success.GetCode(), Msg: name})
}

// withLang 把请求头里的语言放进 ctx。
// langx 约定从 ctx 里读 "Accept-Language"，也可以自己用 langx.SetCtxLang 指定。
func withLang(r *http.Request) context.Context {
	return context.WithValue(r.Context(), "Accept-Language", r.Header.Get("Accept-Language"))
}

// toResponse 是所有 error 的唯一出口：errorx 错误取业务码，其它错误按 500 处理
func toResponse(ctx context.Context, err error) Response {
	var ei errorx.ErrorInterface
	if errors.As(err, &ei) {
		return Response{Code: ei.GetCode(), Msg: ei.Msg(ctx)}
	}
	return Response{Code: errorx.ErrInternal.GetCode(), Msg: errorx.ErrInternal.Msg(ctx)}
}

func writeJSON(w http.ResponseWriter, resp Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	fmt.Println("=== 同一个接口，不同入参与语言 ===")
	cases := []struct{ id, lang string }{
		{"", "zh"},
		{"404", "zh"},
		{"404", "en"},
		{"500", "en"},
		{"42", "zh"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/user?id="+c.id, nil)
		r.Header.Set("Accept-Language", c.lang)
		w := httptest.NewRecorder()
		userHandler(w, r)
		// 统一返回 HTTP 200，业务状态放在 code 字段（国内常见约定），
		// 需要真实 HTTP 状态码时，可以在 toResponse 里按 code 映射。
		fmt.Printf("GET /user?id=%-4s [%s] -> HTTP %d %s", c.id, c.lang, w.Code, w.Body.String())
	}

	fmt.Println()
	fmt.Println("=== 日志里打印完整信息（带 cause 和调用栈）===")
	r := httptest.NewRequest(http.MethodGet, "/user?id=500", nil)
	_, err := loadUser(withLang(r), "500")
	fmt.Printf("%+v", err)
	fmt.Println()
}
