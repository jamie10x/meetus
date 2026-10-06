package rsvp

import "testing"

func TestCSVFormulaNeutralization(t *testing.T) {
	for _, input := range []string{"=1+1", "+123", "-1", "@SUM(A1)", "\tformula", "\nformula", " =1"} {
		if csvText(input) != "'"+input {
			t.Errorf("not neutralized: %q", input)
		}
	}
	if csvText("Aziza") != "Aziza" {
		t.Fatal("ordinary name changed")
	}
}
