package helper

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
)

type ResponseMessageLog struct {
	Message string `json:"message"`
}

// Response builds the standard WebResponse. Code 400 keeps the custom
// message passed by the caller (used for validation errors); every other
// code is translated through response_message.json, honouring the
// X-Lang header (ID by default).
func Response(ctx *fiber.Ctx, config *viper.Viper, code int, message string, data any) *WebResponse[interface{}] {
	res := new(WebResponse[interface{}])
	msgs := message
	msg, err := GetResponseMessageJSON()
	if err != nil {
		fmt.Println("Error GetResponseMessageJSON()")
		code = 500
		msgs = "Server Error"
		data = EmptyObject()
	}

	if code != 400 {
		m, ok := msg["CODE_"+fmt.Sprint(code)]
		if !ok {
			code = 500
			m = msg["CODE_500"]
		}

		if strings.EqualFold(fmt.Sprint(ctx.Locals("lang")), "EN") {
			msgs = m.EN
		} else {
			msgs = m.ID
		}

		code = m.Code
	}

	res.Code = code
	res.Message = msgs
	res.Data = data

	return res
}

func MessageLog(code int, errMsg error) ResponseMessageLog {
	var msgs string
	msg, err := GetResponseMessageJSON()
	if err != nil {
		msgs = "master response message tidak ditemukan"
		return ResponseMessageLog{
			Message: msgs,
		}
	}

	if code >= 400 && code < 500 {
		msgs = msg["CODE_"+fmt.Sprint(code)].ID
	} else if code >= 500 {
		msgs = errMsg.Error()
	}

	return ResponseMessageLog{
		Message: msgs,
	}
}
