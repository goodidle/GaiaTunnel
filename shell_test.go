package tunnel_test

import (
	"Gaia/GaiaTunnel"
	"testing"
)

var (
	Content  = "uptime\nsleep 1\nuptime"
	TimeoutTxt = "uptime\nsleep 10"
	FailTxt = "uptime\nsleep 5\nexit 1"
)

func TestShell_Start(t *testing.T) {
	defer t.Logf("\n\n")
	o := tunnel.Shell{ Content: Content }
	o.Init()
	t.Logf("Shell: %s\n", o.String())
	o.Start()
	if o.State == 8 {
		t.Logf("Right: %s", o.String())
	}else{
		t.Fatalf("Wrong: %s", o.String())
	}
}

func TestShell_Timeout(t *testing.T) {
	defer t.Logf("\n\n")
	o := tunnel.Shell{ Content: TimeoutTxt, Timeout: 5 }
	o.Init()
	t.Logf("Shell: %s\n", o.String())
	o.Start()
	if o.State == 5 {
		t.Logf("Right: %s", o.String())
	}else{
		t.Fatalf("Wrong: %s", o.String())
	}
}

func TestShell_Fail(t *testing.T) {
	defer t.Logf("\n\n")
	o := tunnel.Shell{ Content: FailTxt }
	o.Init()
	t.Logf("Shell: %s\n", o.String())
	o.Start()
	if o.State == 6 {
		t.Logf("Right: %s", o.String())
	}else{
		t.Fatalf("Wrong: %s", o.String())
	}
}
