package theme

import "testing"

func TestOrder_HasSixThemes(t *testing.T) {
	want := []string{"Nord", "Dracula", "Gruvbox", "Tokyo Night", "Solarized Dark", "Solarized Light"}
	if len(Order) != len(want) {
		t.Fatalf("Order has %d themes, want %d", len(Order), len(want))
	}
	for i, name := range want {
		if Order[i] != name {
			t.Fatalf("Order[%d] = %q, want %q", i, Order[i], name)
		}
	}
}

func TestNext_CyclesAndWrapsAround(t *testing.T) {
	if got := Next("Nord"); got != "Dracula" {
		t.Fatalf("Next(Nord) = %q, want Dracula", got)
	}
	if got := Next("Solarized Light"); got != "Nord" {
		t.Fatalf("Next(Solarized Light) = %q, want Nord (wrap around)", got)
	}
}

func TestGet_UnknownNameFallsBackToNord(t *testing.T) {
	got := Get("Not A Theme")
	if got.Name != "Nord" {
		t.Fatalf("Get(unknown).Name = %q, want Nord", got.Name)
	}
}

func TestGet_AllOrderNamesResolve(t *testing.T) {
	for _, name := range Order {
		if got := Get(name); got.Name != name {
			t.Fatalf("Get(%q).Name = %q, want %q", name, got.Name, name)
		}
	}
}

func TestThemes_AllHaveDistinctHotColor(t *testing.T) {
	for _, name := range Order {
		th := Get(name)
		if th.Hot == "" {
			t.Errorf("%s: Hot color is empty", name)
		}
		if th.Hot == th.Accent {
			t.Errorf("%s: Hot (%s) must differ from Accent (%s)", name, th.Hot, th.Accent)
		}
		if th.Hot == th.Muted {
			t.Errorf("%s: Hot (%s) must differ from Muted (%s)", name, th.Hot, th.Muted)
		}
	}
}
