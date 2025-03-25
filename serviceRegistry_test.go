package modular

import (
	"testing"
)

func TestRegisterAndGetServices(t *testing.T) {
	testcases := []struct {
		name    string
		service any
	}{
		{
			name:    "service1",
			service: &service1{name: "service1"},
		},
		{
			name:    "service2",
			service: &service2{title: "service2"},
		},
	}

	app := NewApplication(NewStdConfigProvider(testCfg{Str: "testing"}), &logger{t})

	for _, tc := range testcases {
		switch tc.name {
		case "service1":
			RegisterService(app, tc.name, tc.service.(*service1))
		case "service2":
			RegisterService(app, tc.name, tc.service.(*service2))
		}
	}

	// also a test where we're not having to cast
	RegisterService(app, "service3", &service1{name: "service3"})

	for _, tc := range testcases {
		switch tc.name {
		case "service1":
			svc, exists := GetService[service1](app, tc.name)
			if !exists {
				t.Errorf("Service %s not found", tc.name)
			}
			if svc.Name() != "service1" {
				t.Errorf("Service %s not registered correctly", tc.name)
			}
		case "service2":
			svc, exists := GetService[service2](app, tc.name)
			if !exists {
				t.Errorf("Service %s not found", tc.name)
			}
			if svc.Title() != "service2" {
				t.Errorf("Service %s not registered correctly", tc.name)
			}
		}
	}

	svc, exists := GetService[service1](app, "service3")
	if !exists {
		t.Errorf("Service %s not found", "service3")
	}
	if svc.Name() != "service3" {
		t.Errorf("Service %s not registered correctly", "service3")
	}
}

type service1 struct {
	name string
}

func (s *service1) Name() string {
	return s.name
}

type service2 struct {
	title string
}

func (s *service2) Title() string {
	return s.title
}
