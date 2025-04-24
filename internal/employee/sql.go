package employee

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/10664kls/contactqr/internal/pager"
	sq "github.com/Masterminds/squirrel"
)

var ErrEmployeeNotFound = errors.New("employee not found")

type EmployeeQuery struct {
	ID            string    `json:"id" param:"id"`
	Code          string    `json:"code" param:"code"`
	DepartmentID  string    `json:"departmentId" param:"departmentId"`
	PositionID    string    `json:"positionId" param:"positionId"`
	CompanyID     string    `json:"companyId" param:"companyId"`
	CreatedBefore time.Time `json:"createdBefore" param:"createdBefore"`
	CreatedAfter  time.Time `json:"createdAfter" param:"createdAfter"`
	PageToken     string    `json:"pageToken" param:"pageToken"`
	PageSize      uint64    `json:"pageSize" param:"pageSize"`
}

func (q *EmployeeQuery) ToSql() (string, []any, error) {
	and := sq.And{}

	if q.ID != "" {
		and = append(and, sq.Eq{"EID": q.ID})
	}

	if q.Code != "" {
		and = append(and, sq.Eq{"EMPNO": q.Code})
	}

	if q.DepartmentID != "" {
		and = append(and, sq.Eq{"depid": q.DepartmentID})
	}

	if q.PositionID != "" {
		and = append(and, sq.Eq{"poid": q.PositionID})
	}

	if q.CompanyID != "" {
		and = append(and, sq.Eq{"bid": q.CompanyID})
	}
	if !q.CreatedBefore.IsZero() {
		and = append(and, sq.LtOrEq{"createdate": q.CreatedBefore})
	}
	if !q.CreatedAfter.IsZero() {
		and = append(and, sq.GtOrEq{"createdate": q.CreatedAfter})
	}

	if q.PageToken != "" {
		cursor, err := pager.DecodeCursor(q.PageToken)
		if err != nil {
			return "", nil, err
		}
		and = append(and, sq.Expr("EID < ?", cursor.ID))
	}

	return and.ToSql()
}

func listEmployees(ctx context.Context, db *sql.DB, in *EmployeeQuery) ([]*Employee, error) {
	id := fmt.Sprintf("TOP %d EID", pager.Size(in.PageSize))
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	q, args := sq.
		Select(
			id,
			"EMPNO",
			"bid",
			"BranchName",
			"depid",
			"Departname",
			"poid",
			"Positionname",
			"CONCAT(nameeng, ' ', surnameeng) AS display_name",
			"Emails",
			"phone_number",
			"mobile_number",
			"approveby",
			"createdate",
		).
		From("dbo.vm_employee").
		PlaceholderFormat(sq.AtP).
		Where(pred, args...).
		OrderBy("EID DESC").
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	employees := make([]*Employee, 0)
	for rows.Next() {
		var e Employee
		if err := rows.Scan(
			&e.ID,
			&e.Code,
			&e.CompanyID,
			&e.CompanyName,
			&e.DepartmentID,
			&e.DepartmentName,
			&e.PositionID,
			&e.PositionName,
			&e.DisplayName,
			&e.Email,
			&e.Phone,
			&e.Mobile,
			&e.ManagerID,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		employees = append(employees, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return employees, nil
}

func getEmployee(ctx context.Context, db *sql.DB, in *EmployeeQuery) (*Employee, error) {
	in.PageSize = 1
	if in.ID == "" {
		return nil, ErrEmployeeNotFound
	}

	employees, err := listEmployees(ctx, db, in)
	if err != nil {
		return nil, err
	}

	if len(employees) == 0 {
		return nil, ErrEmployeeNotFound
	}

	return employees[0], nil
}
