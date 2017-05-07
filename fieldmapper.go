package envoyds

import (
	"encoding/json"
	"net/http"

	"github.com/mholt/binding"
)

func (r *ServicePostRequest) FieldMap(req *http.Request) binding.FieldMap {
	return binding.FieldMap{
		&r.Ip:              "ip",
		&r.ServiceRepoName: "service_repo_name",
		&r.Port:            "port",
		&r.Revision:        "revision",
		&r.Tags: binding.Field{
			Form: "tags",
			Binder: func(fieldName string, formVals []string, errs binding.Errors) binding.Errors {
				if err := json.Unmarshal([]byte(formVals[0]), &r.Tags); err != nil {
					errs.Add([]string{fieldName}, err.Error(), err.Error())
				}
				return errs
			},
		},
	}
}

func (r *ServiceUpdateLoadBalancingRequest) FieldMap(req *http.Request) binding.FieldMap {
	return binding.FieldMap{
		&r.LoadBalancingWeight: "load_balancing_weight",
	}
}
