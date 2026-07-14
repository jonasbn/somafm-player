package logo

import "testing"

func TestFor_DroneZoneReturnsDroneZoneArt(t *testing.T) {
	lines, isDefault := For("Drone Zone")
	if isDefault {
		t.Fatal("For(\"Drone Zone\") isDefault = true, want false")
	}
	if len(lines) != len(droneZoneArt) || lines[0] != droneZoneArt[0] {
		t.Fatalf("For(\"Drone Zone\") = %v, want droneZoneArt", lines)
	}
}

func TestFor_DroneZone2ReturnsSameArtAsDroneZone(t *testing.T) {
	lines, isDefault := For("Drone Zone 2")
	if isDefault {
		t.Fatal("For(\"Drone Zone 2\") isDefault = true, want false")
	}
	if len(lines) != len(droneZoneArt) || lines[0] != droneZoneArt[0] {
		t.Fatalf("For(\"Drone Zone 2\") = %v, want droneZoneArt", lines)
	}
}

func TestFor_DeepSpaceOneReturnsDeepSpaceOneArt(t *testing.T) {
	lines, isDefault := For("Deep Space One")
	if isDefault {
		t.Fatal("For(\"Deep Space One\") isDefault = true, want false")
	}
	if len(lines) != len(deepSpaceOneArt) || lines[0] != deepSpaceOneArt[0] {
		t.Fatalf("For(\"Deep Space One\") = %v, want deepSpaceOneArt", lines)
	}
}

func TestFor_UnmatchedTitleReturnsDefaultArt(t *testing.T) {
	for _, title := range []string{"", "Groove Salad", "Drone Zone 3", "not a channel"} {
		lines, isDefault := For(title)
		if !isDefault {
			t.Errorf("For(%q) isDefault = false, want true", title)
		}
		if len(lines) != len(defaultArt) || lines[0] != defaultArt[0] {
			t.Errorf("For(%q) = %v, want defaultArt", title, lines)
		}
	}
}

func TestWidth_ReturnsWidestLineLength(t *testing.T) {
	lines := []string{"ab", "abcde", "abc"}
	if got := Width(lines); got != 5 {
		t.Fatalf("Width(lines) = %d, want 5", got)
	}
}

func TestWidth_EmptySliceReturnsZero(t *testing.T) {
	if got := Width(nil); got != 0 {
		t.Fatalf("Width(nil) = %d, want 0", got)
	}
}

func TestArt_MeasuredWidthsMatchTODOSource(t *testing.T) {
	// docs/TODO.md's ASCII art was measured (via `awk '{print length}'`) at
	// these exact widths when the spec was written. This guards against a
	// future edit to the art silently changing dimensions the rendering
	// logic (internal/ui/logo.go's width-fallback check) depends on.
	if got := Width(defaultArt); got != 44 {
		t.Errorf("Width(defaultArt) = %d, want 44", got)
	}
	if got := Width(droneZoneArt); got != 56 {
		t.Errorf("Width(droneZoneArt) = %d, want 56", got)
	}
	if got := Width(deepSpaceOneArt); got != 61 {
		t.Errorf("Width(deepSpaceOneArt) = %d, want 61", got)
	}
}
