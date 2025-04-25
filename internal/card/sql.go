package card

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/10664kls/contactqr/internal/pager"
	"github.com/10664kls/contactqr/internal/utils"
	sq "github.com/Masterminds/squirrel"
)

var ErrCardNotFound = errors.New("card not found")

type CardQuery struct {
	managerID     int64
	EmployeeID    int64     `json:"employeeId" query:"employeeId"`
	PositionID    int64     `json:"positionId" query:"positionId"`
	DepartmentID  int64     `json:"departmentId" query:"departmentId"`
	CompanyID     int64     `json:"companyId" query:"companyId"`
	ID            string    `json:"id" param:"id" query:"id"`
	DisplayName   string    `json:"displayName" query:"displayName"`
	Status        status    `json:"status" query:"status"`
	CreatedAfter  time.Time `json:"createdAfter" query:"createdAfter"`
	CreatedBefore time.Time `json:"createdBefore" query:"createdBefore"`
	PageToken     string    `json:"pageToken" query:"pageToken"`
	PageSize      uint64    `json:"pageSize" query:"pageSize"`
}

func (q *CardQuery) ToSql() (string, []any, error) {
	and := sq.And{}

	if q.ID != "" {
		and = append(and, sq.Eq{"id": q.ID})
	}

	if q.EmployeeID > 0 {
		and = append(and, sq.Eq{"employee_id": q.EmployeeID})
	}

	if q.DisplayName != "" {
		and = append(and, sq.Expr("display_name LIKE ?", "%"+q.DisplayName+"%"))
	}

	if q.PositionID > 0 {
		and = append(and, sq.Eq{"position_id": q.PositionID})
	}

	if q.DepartmentID > 0 {
		and = append(and, sq.Eq{"department_id": q.DepartmentID})
	}

	if q.CompanyID > 0 {
		and = append(and, sq.Eq{"company_id": q.CompanyID})
	}

	if q.Status != StatusUnspecified {
		and = append(and, sq.Eq{"status": q.Status})
	}

	if q.managerID > 0 {
		and = append(and, sq.Eq{"manager_id": q.managerID})
	}

	if !q.CreatedBefore.IsZero() {
		and = append(and, sq.LtOrEq{"created_at": q.CreatedBefore})
	}
	if !q.CreatedAfter.IsZero() {
		and = append(and, sq.GtOrEq{"created_at": q.CreatedAfter})
	}

	if q.PageToken != "" {
		cursor, err := pager.DecodeCursor(q.PageToken)
		if err != nil {
			return "", nil, err
		}
		and = append(and, sq.Expr("created_at < ?", cursor.Time))
	}

	return and.ToSql()
}

func listCards(ctx context.Context, db *sql.DB, in *CardQuery) ([]*Card, error) {
	id := fmt.Sprintf("TOP %d id", pager.Size(in.PageSize))
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	q, args := sq.
		Select(
			id,
			"employee_id",
			"department_id",
			"position_id",
			"company_id",
			"display_name",
			"employee_code",
			"department_name",
			"position_name",
			"company_name",
			"email",
			"phone",
			"mobile",
			"status",
			"remark",
			"created_at",
			"updated_at",
			"created_by",
			"updated_by",
		).
		From("dbo.v_business_card").
		Where(pred, args...).
		OrderBy("created_at DESC").
		PlaceholderFormat(sq.AtP).
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	cards := make([]*Card, 0)
	for rows.Next() {
		var c Card
		if err := rows.Scan(
			&c.ID,
			&c.EmployeeID,
			&c.DepartmentID,
			&c.PositionID,
			&c.CompanyID,
			&c.DisplayName,
			&c.EmployeeCode,
			&c.DepartmentName,
			&c.PositionName,
			&c.CompanyName,
			&c.Email,
			&c.PhoneNumber,
			&c.MobileNumber,
			&c.Status,
			&c.Remark,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.createdBy,
			&c.updatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		cards = append(cards, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return cards, nil
}

func getCard(ctx context.Context, db *sql.DB, in *CardQuery) (*Card, error) {
	in.PageSize = 1
	if in.ID == "" {
		return nil, ErrCardNotFound
	}

	cards, err := listCards(ctx, db, in)
	if err != nil {
		return nil, err
	}

	if len(cards) == 0 {
		return nil, ErrCardNotFound
	}

	return cards[0], nil
}

func createCard(ctx context.Context, db *sql.DB, in *Card) error {
	return utils.WithTx(ctx, db, func(ctx context.Context, tx *sql.Tx) error {
		q, args := sq.
			Insert("dbo.business_card").
			Columns(
				"id",
				"employee_id",
				"position_id",
				"department_id",
				"company_id",
				"display_name",
				"email",
				"phone",
				"mobile",
				"status",
				"remark",
				"created_at",
				"updated_at",
				"created_by",
				"updated_by",
			).
			Values(
				in.ID,
				in.EmployeeID,
				in.PositionID,
				in.DepartmentID,
				in.CompanyID,
				in.DisplayName,
				in.Email,
				in.PhoneNumber,
				in.MobileNumber,
				in.Status,
				in.Remark,
				in.CreatedAt,
				in.UpdatedAt,
				in.createdBy,
				in.updatedBy,
			).
			PlaceholderFormat(sq.AtP).
			MustSql()

		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("failed to execute create card: %w", err)
		}

		query, args := sq.
			Update("dbo.tb_employee").
			Set("phone_number", in.PhoneNumber).
			Set("mobile_number", in.MobileNumber).
			Where(
				sq.Eq{
					"eid": in.EmployeeID,
				},
			).
			PlaceholderFormat(sq.AtP).
			MustSql()

		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to execute update employee: %w", err)
		}

		return nil
	})
}

func updateCard(ctx context.Context, db *sql.DB, in *Card) error {
	q, args := sq.
		Update("dbo.business_card").
		Set("display_name", in.DisplayName).
		Set("position_id", in.PositionID).
		Set("department_id", in.DepartmentID).
		Set("company_id", in.CompanyID).
		Set("email", in.Email).
		Set("phone", in.PhoneNumber).
		Set("mobile", in.MobileNumber).
		Set("status", in.Status).
		Set("remark", in.Remark).
		Set("updated_at", in.UpdatedAt).
		Set("updated_by", in.updatedBy).
		Where(
			sq.Eq{
				"id": in.ID,
			}).
		PlaceholderFormat(sq.AtP).
		MustSql()

	if _, err := db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}
