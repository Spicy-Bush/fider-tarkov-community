package postgres_test

import (
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/models/cmd"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/enum"
	"github.com/Spicy-Bush/fider-tarkov-community/app/models/query"

	. "github.com/Spicy-Bush/fider-tarkov-community/app/pkg/assert"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/bus"
)

func TestTrialingTenantContactsPreserveUserIdentity(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	var expiresAt time.Time
	if err := trx.Scalar(&expiresAt, "SELECT trial_ends_at FROM tenants_billing WHERE tenant_id = 3"); err != nil {
		t.Fatal(err)
	}
	contacts := &query.GetTrialingTenantContacts{TrialExpiresOn: expiresAt}
	if err := bus.Dispatch(ctx, contacts); err != nil {
		t.Fatal(err)
	}
	if len(contacts.Contacts) != 1 || contacts.Contacts[0] == nil {
		t.Fatalf("expected one trial contact, got %+v", contacts.Contacts)
	}

	contact := contacts.Contacts[0]
	if contact.ID != 6 || contact.Name != "Trial Expired" || contact.Email != "trial.expired@trial-expired.com" {
		t.Fatalf("trial contact lost its identity: %+v", contact)
	}
	if contact.Role != enum.RoleAdministrator || contact.Status != enum.UserActive {
		t.Fatalf("trial contact lost its role or status: %+v", contact)
	}
	if contact.Tenant == nil || contact.Tenant.Subdomain != "trial-expired" {
		t.Fatalf("trial contact lost its tenant: %+v", contact.Tenant)
	}
}

func TestLockExpiredTenants_ShouldTriggerForOneTenant(t *testing.T) {
	ctx := SetupDatabaseTest(t)
	defer TeardownDatabaseTest()

	// There is a tenant with an expired trial setup in the seed for the test database.
	q := &cmd.LockExpiredTenants{}

	err := bus.Dispatch(ctx, q)
	Expect(err).IsNil()
	Expect(q.NumOfTenantsLocked).Equals(int64(1))
	Expect(q.TenantsLocked).Equals([]int{3})

}
