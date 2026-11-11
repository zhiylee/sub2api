package distribution

import "testing"

func TestCompareVersions(t *testing.T) {
	for _, pair := range [][2]string{
		{"0.2.21", "0.2.21-zy.1"},
		{"v0.2.21-zy.1", "0.2.21-zy.2"},
		{"0.2.21-zy.2", "0.2.21-zy.10"},
		{"0.2.21-zy.10", "0.2.22-zy.1"},
		{"0.2.21-zy.999999999999999999999999", "0.2.22"},
		{"0.2.21-zy.999999999999999999999999", "0.2.21-zy.1000000000000000000000000"},
	} {
		t.Run(pair[0]+"_before_"+pair[1], func(t *testing.T) {
			if order, ok := CompareVersions(pair[0], pair[1]); !ok || order >= 0 {
				t.Fatalf("forward order = %d, valid = %v", order, ok)
			}
			if order, ok := CompareVersions(pair[1], pair[0]); !ok || order <= 0 {
				t.Fatalf("reverse order = %d, valid = %v", order, ok)
			}
		})
	}
	if order, ok := CompareVersions("v0.2.21-zy.1", "0.2.21-zy.1"); !ok || order != 0 {
		t.Fatalf("same revision = %d, valid = %v", order, ok)
	}
}

func TestReleaseTags(t *testing.T) {
	for _, tag := range []string{"v0.2.21-zy.1", "v0.2.21-zy.10"} {
		if !IsReleaseTag(tag) || IsUpstreamReleaseTag(tag) {
			t.Errorf("downstream tag rejected: %s", tag)
		}
	}
	if !IsUpstreamReleaseTag("v0.2.21") || IsReleaseTag("v0.2.21") {
		t.Fatal("upstream and downstream channels overlap")
	}
	for _, tag := range []string{
		"", "dev", "0.2.21-zy.1", "v0.2.21-zy.0", "v0.2.21-zy.01", "v00.2.21-zy.1",
		"v0.2.21-rc.1", "v0.2.21-zy.1-beta", "v0.2.21-zy.1+build", "v0.2.21.1-zy.1", " v0.2.21-zy.1",
	} {
		if IsReleaseTag(tag) || IsUpstreamReleaseTag(tag) {
			t.Errorf("invalid release accepted: %s", tag)
		}
	}
	if _, ok := CompareVersions("dev", "0.2.21-zy.1"); ok {
		t.Fatal("unknown build version must not be ordered")
	}
}
