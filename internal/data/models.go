package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Movies      MovieModel
	Usr         UserModel
	Tkn         TokenModel
	Permissions PermissionModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Movies:      MovieModel{BD: db},
		Usr:         UserModel{DB: db},
		Tkn:         TokenModel{DB: db},
		Permissions: PermissionModel{DB: db},
	}
}
