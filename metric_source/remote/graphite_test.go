package remote

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moira-alert/moira/metric_source"
	. "github.com/smartystreets/goconvey/convey"
)

func TestIsConfigured(t *testing.T) {
	Convey("Graphite is not configured", t, func() {
		remote := Create(&Config{URL: "", Enabled: true})
		isConfigured, err := remote.IsConfigured()
		So(isConfigured, ShouldBeFalse)
		So(err, ShouldResemble, ErrRemoteStorageDisabled)
	})

	Convey("Graphite is configured", t, func() {
		remote := Create(&Config{URL: "http://host", Enabled: true})
		isConfigured, err := remote.IsConfigured()
		So(isConfigured, ShouldBeTrue)
		So(err, ShouldBeEmpty)
	})
}

func TestIsRemoteAvailable(t *testing.T) {
	Convey("Is available", t, func() {
		server := createServer([]byte("Some string"), http.StatusOK)
		remote := Graphite{client: server.Client(), config: &Config{URL: server.URL}}
		isAvailable, err := remote.IsRemoteAvailable()
		So(isAvailable, ShouldBeTrue)
		So(err, ShouldBeEmpty)
	})

	Convey("Not available", t, func() {
		server := createServer([]byte("Some string"), http.StatusInternalServerError)
		remote := Graphite{client: server.Client(), config: &Config{URL: server.URL}}
		isAvailable, err := remote.IsRemoteAvailable()
		So(isAvailable, ShouldBeFalse)
		So(err, ShouldResemble, fmt.Errorf("bad response status %d: %s", http.StatusInternalServerError, "Some string"))
	})
}

func TestFetch(t *testing.T) {
	var from int64 = 300
	var until int64 = 500
	target := "foo.bar"

	Convey("Request success but body is invalid", t, func() {
		server := createServer([]byte("[]"), http.StatusOK)
		remote := Graphite{client: server.Client(), config: &Config{URL: server.URL}}
		result, err := remote.Fetch(target, from, until, false)
		So(result, ShouldResemble, &FetchResult{MetricsData: []*metricSource.MetricData{}})
		So(err, ShouldBeEmpty)
	})

	Convey("Request success but body is invalid", t, func() {
		server := createServer([]byte("Some string"), http.StatusOK)
		remote := Graphite{client: server.Client(), config: &Config{URL: server.URL}}
		result, err := remote.Fetch(target, from, until, false)
		So(result, ShouldBeEmpty)
		So(err.Error(), ShouldResemble, "failed to get remote target 'foo.bar': invalid character 'S' looking for beginning of value")
	})

	Convey("Fail request with InternalServerError", t, func() {
		server := createServer([]byte("Some string"), http.StatusInternalServerError)
		remote := Graphite{client: server.Client(), config: &Config{URL: server.URL}}
		result, err := remote.Fetch(target, from, until, false)
		So(result, ShouldBeEmpty)
		So(err.Error(), ShouldResemble, fmt.Sprintf("failed to get remote target 'foo.bar': bad response status %d: %s", http.StatusInternalServerError, "Some string"))
	})

	Convey("Fail make request", t, func() {
		url := "💩%$&TR"
		remote := Graphite{config: &Config{URL: url}}
		result, err := remote.Fetch(target, from, until, false)
		So(result, ShouldBeEmpty)
		So(err.Error(), ShouldResemble, "failed to get remote target 'foo.bar': parse 💩%$&TR: invalid URL escape \"%$&\"")
	})
}