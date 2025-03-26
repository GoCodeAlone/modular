package modular

import "testing"

type testCfg struct {
	Str string `yaml:"str"`
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
