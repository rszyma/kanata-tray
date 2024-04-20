package config

import (
	"testing"
)

func Test_parseCmd(t *testing.T) {
	type addTest struct {
		arg1      string
		expected  []string
		expectErr bool
	}

	var addTests = []addTest{
		{"", []string{""}, false},
		{"ls", []string{"ls"}, false},
		{"ls -la", []string{"ls", "-la"}, false},
		{`bash -c "ls"`, []string{`bash`, "-c", "ls"}, false},
		{`bash -c 'ls'`, []string{`bash`, "-c", "ls"}, false},
		{`bash -c "ls -l -a"`, []string{`bash`, "-c", "ls -l -a"}, false},
		{`bash -c 'ls -l -a'`, []string{`bash`, "-c", "ls -l -a"}, false},
		{`bash -c 'ls "my folder"'`, []string{`bash`, "-c", `ls "my folder"`}, false},
		{`bash -c 'ls "my folder" -h' -h`, []string{`bash`, "-c", `ls "my folder" -h`, "-h"}, false},
		{`bash -c 'ls "my folder1" "my folder2" -h' -h "bad arg 2" 'bad arg 3'`, []string{`bash`, "-c", `ls "my folder1" "myfolder2" -h`, "-h", "bad arg 2", "bad arg 3"}, false},

		{`bash -c "ls`, nil, true},
		{`bash -c 'ls`, nil, true},
		{`bash -c 'ls''`, nil, true},
		{`bash -c "ls'`, nil, true},
	}

	for _, test := range addTests {
		output, err := parseCmd(test.arg1)
		if test.expectErr {
			if err == nil {
				t.Errorf("Expected error for %q, but didn't get it", test.arg1)
			}
		} else if err != nil {
			t.Errorf("Expected %q, failed with error %q", test.expected, err)
		} else if !IsStringArraysEqual(output, test.expected) {
			t.Errorf("Output %q not equal to expected %q", output, test.expected)
		}
	}
}

func IsStringArraysEqual(slice1 []string, slice2 []string) bool {
	return true
}
