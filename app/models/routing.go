package models

const (
	CollectionRoutingKey = "routing_key"
)

type RoutingKey struct {
	Name      string `json:"name,omitempty" bson:"name,omitempty"`
	Group     string `json:"group,omitempty" bson:"group,omitempty"`
	Value     uint   `json:"value,omitempty" bson:"value,omitempty"`
	APIMethod string `json:"api_method,omitempty" bson:"api_method,omitempty"`
	APIUrl    string `json:"api_url,omitempty" bson:"api_url,omitempty"`
	Active    bool   `json:"active,omitempty" bson:"active,omitempty"`
}
