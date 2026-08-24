package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxRequestBody = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var syntax *json.SyntaxError
		var typeError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntax):
			return errors.New("JSON 语法错误")
		case errors.As(err, &typeError):
			return errors.New("JSON 字段类型错误: " + typeError.Field)
		case errors.Is(err, io.EOF):
			return errors.New("请求体不能为空")
		default:
			return err
		}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求体只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data []byte) {
	w.WriteHeader(status)
	_, _ = io.Copy(w, bytes.NewReader(data))
}
