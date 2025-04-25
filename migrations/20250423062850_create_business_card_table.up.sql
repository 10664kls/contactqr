CREATE TABLE dbo.business_card (
  id VARCHAR(12) NOT NULL PRIMARY KEY,
  employee_id INT NOT NULL,
  department_id INT NOT NULL,
  position_id INT NOT NULL,
  company_id INT NOT NULL,
  display_name TEXT NOT NULL,
  email TEXT NOT NULL DEFAULT '',
  phone TEXT NOT NULL,
  mobile TEXT NOT NULL  DEFAULT '',
  status VARCHAR(15) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'PUBLISHED')),
  remark TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  updated_by TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE dbo.business_card
  ADD CONSTRAINT fk_employee_id FOREIGN KEY (employee_id) REFERENCES dbo.tb_employee(EID),
      CONSTRAINT fk_company_id FOREIGN KEY (company_id) REFERENCES dbo.tb_Branch(BID),
      CONSTRAINT fk_department_id FOREIGN KEY (department_id) REFERENCES dbo.tb_department(DEPID),
      CONSTRAINT fk_position_id FOREIGN KEY (position_id) REFERENCES dbo.tb_position(POID);

ALTER TABLE dbo.tb_employee
  ADD phone_number TEXT NOT NULL DEFAULT '',
      mobile_number TEXT NOT NULL DEFAULT '';