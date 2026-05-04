package repository

const (
	publicVisibleFilter = "visibility = :visibility AND (#status = :released OR #status = :inProduction OR #status = :ended)"

	publicVisibleByCastFilter = "contains(castIds, :personId) AND " + publicVisibleFilter

	publicVisibleByAttributeFilter = "contains(attributeIds, :attributeId) AND " + publicVisibleFilter
)
