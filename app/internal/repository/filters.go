package repository

// Reusable DynamoDB filter expressions for movie/TV "publicly visible"
// scans. These predicates are paired with publicVisibilityValues() and the
// "#status" attribute name placeholder added by callers.
//
// Centralizing them avoids drift between the multiple list endpoints that
// must apply the same visibility/status gating.
const (
	// publicVisibleFilter selects items whose visibility is public AND whose
	// status is RELEASED or IN_PRODUCTION.
	publicVisibleFilter = "visibility = :visibility AND (#status = :released OR #status = :inProduction)"

	// publicVisibleByCastFilter additionally requires the cast list to
	// contain :personId.
	publicVisibleByCastFilter = "contains(castIds, :personId) AND " + publicVisibleFilter

	// publicVisibleByAttributeFilter additionally requires the attribute
	// list to contain :attributeId.
	publicVisibleByAttributeFilter = "contains(attributeIds, :attributeId) AND " + publicVisibleFilter
)
