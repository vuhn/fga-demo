package service

import (
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

func grantOutput(m model.Grant) output.Grant {
	return output.Grant{
		ID:          m.ID,
		TenantID:    m.TenantID,
		ObjectType:  m.ObjectType,
		ObjectID:    m.ObjectID,
		Relation:    m.Relation,
		SubjectType: m.SubjectType,
		UserID:      m.UserID,
		UserName:    m.UserName,
		CreatedAt:   m.CreatedAt,
	}
}
