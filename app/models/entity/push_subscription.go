package entity

import "time"

type PushSubscription struct {
	ID        int       `db:"id"`
	TenantID  int       `db:"tenant_id"`
	UserID    int       `db:"user_id"`
	Endpoint  string    `db:"endpoint"`
	KeyP256dh string    `db:"key_p256dh"`
	KeyAuth   string    `db:"key_auth"`
	CreatedAt time.Time `db:"created_at"`
}

