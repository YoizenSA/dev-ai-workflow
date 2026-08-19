package config

import "testing"

func TestBuiltInDefaults_PonytailOn(t *testing.T) {
	d := BuiltInDefaults()
	if !d.Ponytail {
		t.Fatal("ponytail must be on by default for install")
	}
}
