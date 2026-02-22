package bitrix

import "fmt"

type APIError struct {
	Errors           string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (e APIError) IsZero() bool {
	return e.Errors == "" && e.ErrorDescription == ""
}

func (e APIError) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("bitrix api error: %s (%s)", e.Errors, e.ErrorDescription)
	}
	return fmt.Sprintf("bitrix api error: %s", e.Errors)
}
