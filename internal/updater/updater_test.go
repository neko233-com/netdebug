package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    int
	}{
		{current: "v0.1.0", latest: "v0.1.0", want: 0},
		{current: "0.1.0", latest: "v0.2.0", want: -1},
		{current: "v1.2.0", latest: "v1.1.9", want: 1},
		{current: "dev", latest: "v0.1.0", want: -1},
	}
	for _, test := range tests {
		if got := compareVersions(test.current, test.latest); got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.current, test.latest, got, test.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := assetName("v0.1.0", "linux", "amd64"); got != "netdebug_0.1.0_linux_amd64.tar.gz" {
		t.Fatalf("unexpected Linux asset: %s", got)
	}
	if got := assetName("v0.1.0", "windows", "arm64"); got != "netdebug_0.1.0_windows_arm64.zip" {
		t.Fatalf("unexpected Windows asset: %s", got)
	}
	if got := assetName("v0.1.0", "freebsd", "amd64"); got != "" {
		t.Fatalf("unsupported target returned asset: %s", got)
	}
}

func TestChecksumFor(t *testing.T) {
	contents := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  netdebug_0.1.0_linux_amd64.tar.gz\n"
	if got, err := checksumFor(contents, "netdebug_0.1.0_linux_amd64.tar.gz"); err != nil || got != "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("checksumFor() = %q, %v", got, err)
	}
	if _, err := checksumFor(contents, "missing.tar.gz"); err == nil {
		t.Fatal("checksumFor() accepted missing asset")
	}
}
