package updater

import (
	"fmt"
	"strconv"
	"strings"
)

type version struct {
	major, minor, patch int
	pre                 []string
}

func parseVersion(raw string) (version, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if i := strings.IndexByte(raw, '+'); i >= 0 {
		raw = raw[:i]
	}
	base, pre, _ := strings.Cut(raw, "-")
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("phiên bản %q không đúng dạng X.Y.Z", raw)
	}
	var out version
	values := []*int{&out.major, &out.minor, &out.patch}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return version{}, fmt.Errorf("phiên bản %q không hợp lệ", raw)
		}
		*values[i] = n
	}
	if pre != "" {
		out.pre = strings.Split(pre, ".")
	}
	return out, nil
}

func compareVersion(a, b version) int {
	av := []int{a.major, a.minor, a.patch}
	bv := []int{b.major, b.minor, b.patch}
	for i := range av {
		if av[i] < bv[i] {
			return -1
		}
		if av[i] > bv[i] {
			return 1
		}
	}
	if len(a.pre) == 0 && len(b.pre) == 0 {
		return 0
	}
	if len(a.pre) == 0 {
		return 1
	}
	if len(b.pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if a.pre[i] == b.pre[i] {
			continue
		}
		an, ae := strconv.Atoi(a.pre[i])
		bn, be := strconv.Atoi(b.pre[i])
		switch {
		case ae == nil && be == nil:
			if an < bn {
				return -1
			}
			return 1
		case ae == nil:
			return -1
		case be == nil:
			return 1
		default:
			if a.pre[i] < b.pre[i] {
				return -1
			}
			return 1
		}
	}
	if len(a.pre) < len(b.pre) {
		return -1
	}
	if len(a.pre) > len(b.pre) {
		return 1
	}
	return 0
}
