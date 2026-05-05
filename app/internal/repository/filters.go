package repository

const (
	publicVisibleFilter = "visibility = :visibility AND (#status = :released OR #status = :inProduction OR #status = :ended)"

	publicVisibleByCastFilter = "contains(castIds, :personId) AND " + publicVisibleFilter
)
