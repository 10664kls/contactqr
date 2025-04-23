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
	Number        string    `json:"number" param:"number"`
	Department    string    `json:"department" param:"department"`
	JobTitle      string    `json:"jobTitle" param:"jobTitle"`
	Company       string    `json:"company" param:"company"`
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

	if q.Number != "" {
		and = append(and, sq.Eq{"EMPNO": q.Number})
	}

	if q.Department != "" {
		and = append(and, sq.Eq{"Departname": q.Department})
	}

	if q.JobTitle != "" {
		and = append(and, sq.Eq{"Positionname": q.JobTitle})
	}

	if q.Company != "" {
		and = append(and, sq.Eq{"BranchName": q.Company})
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
			"BranchName",
			"Departname",
			"Positionname",
			"nameeng",
			"surnameeng",
			"Emails",
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
			&e.Number,
			&e.Company,
			&e.Department,
			&e.JobTitle,
			&e.FirstName,
			&e.LastName,
			&e.Contact.Email,
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
