package expectation

import "errors"

var (
	BotCommandAnswerError = errors.New("command_answer_error")
	BotMessageAnswerError = errors.New("message_answer_error")
	ClientRequestError    = errors.New("client_request_error")
)
