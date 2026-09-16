# ERP API Architecture

## 1. Overview

Backend สำหรับ ERP ใช้:

- Go
- Fiber
- Clean Architecture
- REST API
- PostgreSQL
- JWT Authentication
- Repository Pattern
- DTO แยกจาก Domain / Persistence Model
- Centralized Error Handling
- Centralized Response Format
- Transaction สำหรับ Business Operation ที่เกี่ยวข้องกับ Stock / Order / Invoice

เป้าหมายหลักคือ:

1. แยก HTTP Layer ออกจาก Business Logic
2. แยก Business Logic ออกจาก Data Access
3. ทำให้ Business Domain ไม่ผูกกับ Framework หรือ Database
4. ทำให้แต่ละ Layer สามารถทดสอบและ Refactor ได้ง่าย
5. ลด Code Duplication
6. ให้ Business Rule สำคัญอยู่ที่ Backend เป็น Source of Truth

---

# 2. Architecture Style

โปรเจกต์นี้ใช้แนวทาง:

> **Clean Architecture with Repository Pattern**

โดยแยกโครงสร้างหลักออกเป็น 4 ส่วนภายใน `internal`

```text
Delivery
   ↓
Usecase
   ↓
Repository
   ↓
Database
```

โดย `Domain` จะเป็นแกนกลางของ Business Model และ Business Rule

```text
                ┌───────────────┐
                │   Delivery    │
                │ HTTP / Fiber  │
                └───────┬───────┘
                        ↓
                ┌───────────────┐
                │    Usecase    │
                │ Business Flow │
                └───────┬───────┘
                        ↓
                ┌───────────────┐
                │  Repository   │
                │  Data Access  │
                └───────┬───────┘
                        ↓
                ┌───────────────┐
                │   Database    │
                └───────────────┘

          Domain = Core Business Model / Rule
```

---

# 3. Recommended Project Structure

```text
backend/
│
├── cmd/
│   └── api/
│       └── main.go
│
├── config/
│   ├── .gitkeep
│   └── config.go
│
├── docs/
│
├── internal/
│   │
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── handler/
│   │   │   ├── middleware/
│   │   │   ├── route/
│   │   │   └── dto/
│   │   │
│   │   └── response/
│   │
│   ├── domain/
│   │   ├── auth/
│   │   ├── user/
│   │   ├── product/
│   │   ├── sku/
│   │   ├── inventory/
│   │   ├── warehouse/
│   │   ├── supplier/
│   │   ├── customer/
│   │   ├── purchase/
│   │   ├── sales/
│   │   ├── order/
│   │   ├── invoice/
│   │   ├── payment/
│   │   └── report/
│   │
│   ├── repository/
│   │   ├── postgres/
│   │   └── redis/
│   │
│   └── usecase/
│       ├── auth/
│       ├── user/
│       ├── product/
│       ├── sku/
│       ├── inventory/
│       ├── warehouse/
│       ├── supplier/
│       ├── customer/
│       ├── purchase/
│       ├── sales/
│       ├── order/
│       ├── invoice/
│       ├── payment/
│       └── report/
│
├── pkg/
│   ├── database/
│   ├── jwt/
│   ├── validator/
│   ├── logger/
│   ├── errors/
│   └── utils/
│
├── migrations/
├── go.mod
└── go.sum
```

---

# 4. Responsibility of Each Layer

| Layer      | Responsibility                                                        |
| ---------- | --------------------------------------------------------------------- |
| Delivery   | HTTP / REST API, Handler, Router, Middleware, Request / Response      |
| Domain     | Core Business Entity, Domain Model, Repository Contract, Domain Error |
| Usecase    | Business Logic, Workflow, Transaction, Business Rule                  |
| Repository | Database / External Data Access Implementation                        |
| pkg        | Shared Infrastructure / Utility                                       |
| config     | Application Configuration                                             |

---

# 5. Request Flow

มาตรฐาน Request Flow:

```text
Client
   ↓
Fiber Router
   ↓
Middleware
   ↓
Delivery / Handler
   ↓
Request DTO / Validation
   ↓
Usecase
   ↓
Repository Interface
   ↓
Repository Implementation
   ↓
PostgreSQL / External Service
```

เมื่อส่ง Response:

```text
Database
   ↓
Repository
   ↓
Usecase
   ↓
Delivery / Handler
   ↓
Standard Response
   ↓
Client
```

---

# 6. Dependency Direction

หลักสำคัญของ Clean Architecture คือ Business Logic ไม่ควรผูกกับ HTTP Framework หรือ Database โดยตรง

```text
Delivery
   ↓
Usecase
   ↓
Domain

Repository
   ↓
Domain
```

Repository Implementation สามารถเปลี่ยนได้โดยไม่กระทบ Usecase

ตัวอย่าง:

```text
Usecase
   ↓
SKURepository Interface
   ↑
   │
PostgresSKURepository
```

ดังนั้นสามารถเปลี่ยนจาก PostgreSQL เป็น Database Implementation อื่นได้ โดยไม่ต้องแก้ Business Logic หลัก

---

# 7. Delivery Layer

Delivery Layer รับผิดชอบเรื่องการสื่อสารกับภายนอก

```text
internal/delivery/
├── http/
│   ├── handler/
│   ├── middleware/
│   ├── route/
│   └── dto/
└── response/
```

หน้าที่:

- Register Route
- รับ HTTP Request
- Parse Request
- Validate Request Format
- เรียก Usecase
- แปลงผลลัพธ์เป็น HTTP Response
- จัดการ HTTP Context

## Delivery ไม่ควร

```text
Handler → SQL
Handler → Business Logic
Handler → Transaction
Handler → Stock Calculation
Handler → Order Workflow
```

ตัวอย่าง Handler:

```go
func (h *SKUHandler) Create(c *fiber.Ctx) error {
    var req dto.CreateSKURequest

    if err := c.BodyParser(&req); err != nil {
        return response.BadRequest(c, err)
    }

    if err := h.validator.Struct(req); err != nil {
        return response.ValidationError(c, err)
    }

    result, err := h.usecase.Create(c.Context(), req)
    if err != nil {
        return err
    }

    return response.Created(c, result)
}
```

---

# 8. Domain Layer

Domain เป็นแกนกลางของระบบ

```text
internal/domain/
├── auth/
├── user/
├── product/
├── sku/
├── inventory/
├── warehouse/
├── supplier/
├── customer/
├── purchase/
├── sales/
├── order/
├── invoice/
├── payment/
└── report/
```

ตัวอย่าง Domain `sku`:

```text
internal/domain/sku/
├── entity.go
├── repository.go
└── errors.go
```

ตัวอย่าง Entity:

```go
type SKU struct {
    ID        int64
    SKU       string
    Name      string
    Price     decimal.Decimal
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

ตัวอย่าง Repository Contract:

```go
type Repository interface {
    Create(ctx context.Context, sku SKU) (*SKU, error)
    FindByID(ctx context.Context, id int64) (*SKU, error)
    FindAll(ctx context.Context, query Query) ([]SKU, error)
    ExistsBySKU(ctx context.Context, sku string) (bool, error)
    Update(ctx context.Context, sku SKU) (*SKU, error)
    Delete(ctx context.Context, id int64) error
}
```

Domain ไม่ควรผูกกับ:

- Fiber
- HTTP Request / Response
- PostgreSQL Driver
- SQL Query โดยตรง

---

# 9. Usecase Layer

Usecase เป็นศูนย์กลางของ Business Logic และ Business Workflow

```text
internal/usecase/
├── auth/
├── user/
├── product/
├── sku/
├── inventory/
├── warehouse/
├── supplier/
├── customer/
├── purchase/
├── sales/
├── order/
├── invoice/
├── payment/
└── report/
```

ตัวอย่าง:

```text
internal/usecase/sku/
├── create.go
├── update.go
├── get.go
└── delete.go
```

Usecase ควรรับผิดชอบ:

- Business Rule
- Business Validation
- Workflow
- Transaction
- การเรียก Repository หลายตัว
- Domain Error
- Stock Calculation
- Order Workflow
- Invoice Workflow

ตัวอย่าง:

```go
type CreateSKUUsecase struct {
    repo sku.Repository
}

func (u *CreateSKUUsecase) Execute(
    ctx context.Context,
    req CreateSKUInput,
) (*sku.SKU, error) {

    exists, err := u.repo.ExistsBySKU(ctx, req.SKU)
    if err != nil {
        return nil, err
    }

    if exists {
        return nil, sku.ErrAlreadyExists
    }

    entity := sku.SKU{
        SKU:  req.SKU,
        Name: req.Name,
    }

    return u.repo.Create(ctx, entity)
}
```

---

# 10. Repository Layer

Repository Layer รับผิดชอบ Data Access Implementation

```text
internal/repository/
├── postgres/
│   ├── auth_repository.go
│   ├── user_repository.go
│   ├── sku_repository.go
│   ├── product_repository.go
│   ├── inventory_repository.go
│   ├── order_repository.go
│   └── invoice_repository.go
│
└── redis/
    └── cache_repository.go
```

ตัวอย่าง:

```go
type PostgresSKURepository struct {
    db *sql.DB
}

func (r *PostgresSKURepository) Create(
    ctx context.Context,
    entity sku.SKU,
) (*sku.SKU, error) {

    // SQL / PostgreSQL implementation

    return &entity, nil
}
```

Repository ควรทำเฉพาะ:

- SQL Query
- Insert / Update / Delete
- Query Data
- Database Mapping
- Persistence

Repository ไม่ควรทำ:

```text
Repository → Business Workflow
Repository → Stock Business Rule
Repository → HTTP Response
```

---

# 11. DTO Rules

DTO อยู่ใน Delivery Layer เพราะเป็น API Contract

```text
internal/delivery/http/dto/
├── auth.go
├── sku.go
├── product.go
├── inventory.go
├── order.go
└── invoice.go
```

ห้ามใช้ Database / Persistence Model เป็น API Contract โดยตรง

## Request DTO

```go
type CreateSKURequest struct {
    SKU   string `json:"sku" validate:"required"`
    Name  string `json:"name" validate:"required"`
    Price string `json:"price" validate:"required"`
}
```

## Response DTO

```go
type SKUResponse struct {
    ID    int64  `json:"id"`
    SKU   string `json:"sku"`
    Name  string `json:"name"`
    Price string `json:"price"`
}
```

Flow:

```text
HTTP Request DTO
      ↓
Delivery Mapping
      ↓
Usecase Input
      ↓
Domain Entity
      ↓
Repository
```

---

# 12. API Versioning

ใช้:

```text
/api/v1/
```

ตัวอย่าง:

```text
POST   /api/v1/auth/login

GET    /api/v1/products
POST   /api/v1/products
GET    /api/v1/products/:id
PUT    /api/v1/products/:id
DELETE /api/v1/products/:id

GET    /api/v1/skus
POST   /api/v1/skus
GET    /api/v1/skus/:id
PUT    /api/v1/skus/:id
DELETE /api/v1/skus/:id

GET    /api/v1/inventory/stocks
GET    /api/v1/inventory/movements

GET    /api/v1/orders
POST   /api/v1/orders
GET    /api/v1/orders/:id

POST   /api/v1/invoices
GET    /api/v1/invoices/:id
```

---

# 13. Standard Response

กำหนด Response Format กลางเพื่อให้ Frontend จัดการง่าย

## Success

```json
{
  "success": true,
  "data": {},
  "message": "Success"
}
```

## List

```json
{
  "success": true,
  "data": [],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 120
  }
}
```

## Error

```json
{
  "success": false,
  "error": {
    "code": "SKU_ALREADY_EXISTS",
    "message": "SKU already exists"
  }
}
```

---

# 14. Error Handling

Domain Error ควรอยู่ใน Domain Layer เช่น:

```text
ErrNotFound
ErrUnauthorized
ErrForbidden
ErrValidation
ErrConflict
ErrSKUAlreadyExists
ErrInsufficientStock
ErrInvalidOrderStatus
```

Flow:

```text
Domain / Repository Error
          ↓
        Usecase
          ↓
Central Error Handler
          ↓
HTTP Status + Standard JSON
```

ไม่ควรเขียน Response Error หลายรูปแบบในแต่ละ Handler

---

# 15. Authentication / Authorization Flow

```text
POST /api/v1/auth/login
        ↓
Delivery / Handler
        ↓
Auth Usecase
        ↓
Verify User
        ↓
Verify Password
        ↓
Generate Access Token
        ↓
Client
        ↓
Authorization: Bearer <token>
        ↓
JWT Middleware
        ↓
Permission Check
        ↓
Delivery / Handler
        ↓
Usecase
```

แยกให้ชัด:

```text
Authentication
= คุณเป็นใคร?

Authorization
= คุณมีสิทธิ์ทำอะไร?
```

---

# 16. Inventory / Bundle Business Rule

ERP มี Business Rule สำคัญเรื่อง SKU และ Package / Bundle

Business Logic นี้ต้องอยู่ใน Usecase Layer

```text
Order Usecase
      ↓
Order Item
      ↓
SKU
      ↓
Is Bundle?
  ├── No
  │    ↓
  │  Deduct SKU
  │
  └── Yes
       ↓
      Load Bundle Items
       ↓
      SKU A × Qty
      SKU B × Qty
      SKU C × Qty
       ↓
      Check Stock
       ↓
      Deduct Stock
       ↓
      Create Stock Movement
```

ตัวอย่าง Package:

```text
FD-MX-C1T2-03
├── FD-TN-80-01 × 1
├── FD-CK-80-01 × 1
└── FD-RL-80-10 × 1
```

Business Rule นี้ต้องอยู่ใน Backend Usecase

ไม่ควรให้:

```text
Frontend → ตัด Stock
Handler  → ตัด Stock
Repository → ตัดสินใจ Business Rule
```

---

# 17. Transaction

กระบวนการที่แก้ไขข้อมูลหลายส่วนต้องใช้ Transaction

Transaction ควรถูกควบคุมจาก Usecase Layer

ตัวอย่าง Shipment Usecase:

```text
BEGIN

1. Load Order
2. Validate Order Status
3. Load Order Items
4. Resolve Bundle
5. Check Stock
6. Deduct Stock
7. Create Stock Movement
8. Update Order Status
9. Create Shipment

COMMIT
```

ถ้าเกิด Error:

```text
ROLLBACK
```

เป้าหมายคือไม่ให้เกิดสถานะข้อมูลไม่สอดคล้องกัน เช่น:

```text
Stock ถูกตัดแล้ว
แต่
Order ไม่เปลี่ยนสถานะ
```

---

# 18. Route Structure

Route อยู่ใน Delivery Layer

```text
internal/delivery/http/
├── handler/
├── middleware/
├── route/
└── dto/
```

ตัวอย่าง:

```go
func RegisterSKURoutes(
    app fiber.Router,
    handler *handler.SKUHandler,
) {
    sku := app.Group("/api/v1/skus")

    sku.Get("/", handler.GetAll)
    sku.Get("/:id", handler.GetByID)
    sku.Post("/", handler.Create)
    sku.Put("/:id", handler.Update)
    sku.Delete("/:id", handler.Delete)
}
```

---

# 19. Clean Architecture Rules

## ห้าม

```text
Delivery / Handler → SQL
Delivery / Handler → Business Logic
Delivery / Handler → Transaction

Frontend → Database
Frontend → ตัด Stock

Usecase → HTTP Response

Repository → Business Workflow
Repository → Business Decision
```

## ควร

```text
Delivery
   ↓
Usecase
   ↓
Domain Repository Contract
   ↑
Repository Implementation
   ↓
Database
```

Responsibility:

```text
HTTP Logic
→ Delivery

Business Logic
→ Usecase

Business Model / Contract
→ Domain

Data Access
→ Repository

API Contract
→ Delivery DTO

Persistence
→ Repository

Cross-cutting
→ Middleware / pkg
```

---

# 20. Testing

ควรแบ่ง Test ตาม Architecture Layer

```text
internal/
├── delivery/
│   └── HTTP / API Test
│
├── domain/
│   └── Domain Rule Test
│
├── usecase/
│   └── Unit Test
│
└── repository/
    └── Integration Test
```

ประเภท Test:

```text
Usecase Unit Test
Repository Integration Test
Delivery / API Test
```

Business Rule สำคัญ เช่น:

```text
Bundle + Stock
Order Workflow
Invoice Workflow
Transaction
```

ควรมี Test โดยเฉพาะ

---

# 21. ERP Domains

Domain ที่แนะนำ:

```text
Auth
├── Login
└── Token

User
├── User
├── Role
└── Permission

Product
├── Product
├── SKU
└── Package / Bundle

Inventory
├── Stock
├── Lot
├── Stock Movement
├── Reservation
└── Adjustment

Warehouse
├── Warehouse
├── Location
└── Transfer

Purchasing
├── Supplier
├── Purchase Order
└── Goods Receipt

Sales
├── Customer
├── Sales Order
└── Shipment

Finance
├── Invoice
└── Payment

Report
```

---

# 22. Final Architecture Flow

ภาพรวมของ ERP API:

```text
┌─────────────────────────────┐
│           CLIENT            │
│     Web / Mobile / Other    │
└──────────────┬──────────────┘
               │ HTTP
               ▼
┌─────────────────────────────┐
│          DELIVERY           │
│                             │
│ Router                      │
│ Middleware                  │
│ Handler                     │
│ DTO                         │
│ Response                    │
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│           USECASE           │
│                             │
│ Business Logic              │
│ Business Workflow           │
│ Transaction                 │
│ Authorization Rule          │
│ Stock Calculation           │
│ Order / Invoice Workflow    │
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│           DOMAIN            │
│                             │
│ Entity                      │
│ Business Model              │
│ Repository Contract         │
│ Domain Error                │
└──────────────┬──────────────┘
               ▲
               │ implements
┌──────────────┴──────────────┐
│         REPOSITORY          │
│                             │
│ PostgreSQL                  │
│ Redis                       │
│ External Data Source        │
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│       DATABASE / INFRA      │
│                             │
│ PostgreSQL                  │
│ Redis                       │
│ External Services           │
└─────────────────────────────┘
```

---

# 23. Architecture Principle

หลักสำคัญ:

> **Go API เป็น Source of Truth ของ Business Logic**

Frontend มีหน้าที่:

- แสดงผล
- รับ Input
- จัดการ State
- เรียก API
- จัดการ User Interaction

Backend มีหน้าที่:

- Validate Business Rule
- Authentication / Authorization
- Transaction
- Stock Calculation
- Order Workflow
- Invoice Workflow
- Data Integrity

และ Flow หลักของระบบคือ:

```text
Client
   ↓
Delivery
   ↓
Usecase
   ↓
Domain
   ↓
Repository
   ↓
Database
```

> หมายเหตุ: ในเชิง Dependency ของ Clean Architecture นั้น `Domain` ไม่ควรขึ้นกับ `Repository Implementation` โดย Repository จะเป็นผู้ Implement Contract ที่ Domain กำหนดไว้
