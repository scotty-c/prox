package util

import (
	"reflect"
	"testing"
)

func TestFindSimilarStringsSuggestsEachNameOnce(t *testing.T) {
	got := FindSimilarStrings("we", []string{"web", "web", "db", "web"}, 3)
	want := []string{"web", "db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
