package profile

import "testing"

func TestValidate(t *testing.T) {
	for _, ok := range []string{"work", "client-a", "A_1"} {
		if err := Validate(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "has space", "a@b", "../x", "this-name-is-far-too-long-for-a-profile"} {
		if err := Validate(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestSelectAndActive(t *testing.T) {
	t.Setenv(EnvVar, "")
	if name, err := Active(); name != "" || err != nil {
		t.Errorf("unset: %q %v", name, err)
	}
	if err := Select("work"); err != nil {
		t.Fatal(err)
	}
	if name, err := Active(); name != "work" || err != nil {
		t.Errorf("selected: %q %v", name, err)
	}
	if err := Select("bad name"); err == nil {
		t.Error("expected Select to reject a bad name")
	}
	t.Setenv(EnvVar, "bad name")
	if _, err := Active(); err == nil {
		t.Error("expected Active to reject a bad env value")
	}
}

func TestKeyAndLabel(t *testing.T) {
	if Key("jira_url", "") != "jira_url" || Key("jira_url", "work") != "jira_url@work" {
		t.Error("Key")
	}
	if Label("") != "default" || Label("work") != "work" {
		t.Error("Label")
	}
}
