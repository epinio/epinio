package models

type ServiceResponse struct {
	Name      string
	BoundApps []string
}

type ServiceResponseList []ServiceResponse
