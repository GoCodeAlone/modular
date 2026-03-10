package modular

import "testing"

type testTypedService struct{ Value string }

func TestRegisterTypedService_and_GetTypedService(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(&struct{}{}), nopLogger{})
	svc := &testTypedService{Value: "hello"}
	if err := RegisterTypedService[*testTypedService](app, "test.svc", svc); err != nil {
		t.Fatalf("RegisterTypedService: %v", err)
	}
	got, err := GetTypedService[*testTypedService](app, "test.svc")
	if err != nil {
		t.Fatalf("GetTypedService: %v", err)
	}
	if got.Value != "hello" {
		t.Errorf("expected hello, got %s", got.Value)
	}
}

func TestGetTypedService_WrongType(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(&struct{}{}), nopLogger{})
	_ = RegisterTypedService[string](app, "str.svc", "hello")
	_, err := GetTypedService[int](app, "str.svc")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestGetTypedService_NotFound(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(&struct{}{}), nopLogger{})
	_, err := GetTypedService[string](app, "missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}
