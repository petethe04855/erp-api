package middleware

import (
	"context"
	"strings"

	domainPermission "chawy-erp-api/internal/domain/permission"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// AccessChecker answers whether a role may perform an action on a page
// (implemented by usecase/permission).
type AccessChecker interface {
	Can(ctx context.Context, role, page string, action domainPermission.Action) (bool, error)
}

// Rule is one page/action pair a route accepts.
type Rule struct {
	Page   string
	Action domainPermission.Action
}

// RequireAccess lets the request through when the user's role holds at least
// one of the rules in the owner-edited permission matrix. Owner always passes.
// A nil checker or a failed lookup denies (fail closed).
func RequireAccess(checker AccessChecker, rules ...Rule) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok {
			return response.Forbidden(c, "Role information missing", "FORBIDDEN")
		}
		if strings.EqualFold(role, domainPermission.OwnerRole) {
			return c.Next()
		}
		if checker == nil {
			return response.Forbidden(c, "You do not have permission to perform this action", "FORBIDDEN")
		}
		for _, r := range rules {
			allowed, err := checker.Can(c.UserContext(), role, r.Page, r.Action)
			if err != nil {
				return response.InternalServerError(c, "Could not check permissions", "PERMISSION_CHECK_FAILED")
			}
			if allowed {
				return c.Next()
			}
		}
		return response.Forbidden(c, "คุณไม่มีสิทธิ์ทำรายการนี้ (กรุณาติดต่อ Owner เพื่อขอสิทธิ์)", "FORBIDDEN")
	}
}
