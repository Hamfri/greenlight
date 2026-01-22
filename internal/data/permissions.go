package data

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/lib/pq"
)

type Permission string

type Permissions []Permission

const (
	PermissionMoviesRead  Permission = "movies:read"
	PermissionMoviesWrite Permission = "movies:write"
)

func (p Permissions) Includes(code Permission) bool {
	return slices.Contains(p, code)
}

type PermissionsModel struct {
	DB *sql.DB
}

func (m PermissionsModel) GetUserPermissions(userID int) (Permissions, error) {
	query := `
		SELECT permissions.code
		FROM permissions
		INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
		INNER JOIN users ON users_permissions.user_id = users.id
		WHERE users.id = $1
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions

	for rows.Next() {
		var permission Permission
		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, permission)
	}
	// check if any errors occurred during the iteration
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

// note ... variadic parameter for codes so that we can assign multiple permissions in a single call
func (m PermissionsModel) AddUserPermissions(userID int, permissions ...Permission) error {
	query := `
		INSERT INTO users_permissions
		SELECT $1, permissions.id FROM permissions WHERE permissions.code = ANY($2)
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID, pq.Array(permissions))
	return err
}
