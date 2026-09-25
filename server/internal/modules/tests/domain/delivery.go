package domain

// DeliveryVersion identifies the immutable dealing algorithm of a published paper.
type DeliveryVersion string

const (
	DeliverySectionV1 DeliveryVersion = "section_v1"
	DeliveryGroupV1   DeliveryVersion = "group_v1"
)
