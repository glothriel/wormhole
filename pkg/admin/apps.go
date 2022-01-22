package admin

import (
	"encoding/json"
	"net/http"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/sirupsen/logrus"
)

type appLister interface {
	Apps() ([]apps.App, error)
}

func listAppsHander(appList appLister) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		theAppList, appListErr := appList.Apps()
		if appListErr != nil {
			rw.WriteHeader(500)
			logrus.Error(appListErr)
			return
		}
		allApps := []appsResponse{}
		for _, app := range theAppList {
			allApps = append(allApps, appsResponse{
				Port: app.Port,
				App:  app.Name,
			})
		}

		appsListAsBytes, marshalErr := json.Marshal(allApps)
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

type appsResponse struct {
	App  string `json:"app"`
	Port int    `json:"port"`
}
