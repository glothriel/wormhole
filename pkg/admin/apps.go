package admin

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
)

type appLister interface {
	Apps() ([]AppListEntry, error)
}

func listAppsHander(appList appLister) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		theAppList, appListErr := appList.Apps()
		if appListErr != nil {
			rw.WriteHeader(500)
			logrus.Error(appListErr)
			return
		}
		appsListAsBytes, marshalErr := json.Marshal(theAppList)
		if marshalErr != nil {
			rw.WriteHeader(500)
			logrus.Error(marshalErr)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(200)
		rw.Write(appsListAsBytes)
	}
}

// AppListEntry represents an entry on app list
type AppListEntry struct {
	App      string `json:"app"`
	Endpoint string `json:"endpoint"`
	Peer     string `json:"peer"`
}
