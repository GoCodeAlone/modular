package modular

import "testing"

type testCfg struct {
	Str string `yaml:"str"`
}

func Test_dynamicConfig_UnmarshalYAML(t *testing.T) {
	type fields struct {
		appCfg    *any
		unmatched map[string]any
	}
	type args struct {
		unmarshal func(interface{}) error
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &dynamicConfig{
				appCfg:    tt.fields.appCfg,
				unmatched: tt.fields.unmatched,
			}
			if err := c.UnmarshalYAML(tt.args.unmarshal); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_hasTaggedField(t *testing.T) {
	type args struct {
		cfg       any
		fieldName string
		tag       string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasMatchingField(tt.args.cfg, tt.args.fieldName, tt.args.tag); got != tt.want {
				t.Errorf("hasMatchingField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_loadAppConfig(t *testing.T) {
	type args struct {
		app *Application
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := loadAppConfig(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("loadAppConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
