package httpjson

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsTrailingValue(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"first"} {"name":"second"}`))
	var target struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(request, &target); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestDecodeJSONRejectsUnknownField(t *testing.T) {
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"first","extra":true}`))
	var target struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(request, &target); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}
