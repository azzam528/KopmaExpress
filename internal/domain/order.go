package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PaymentMethod string

const (
	PayCash     PaymentMethod = "CASH"
	PayTransfer PaymentMethod = "TRANSFER"
	PayQRIS     PaymentMethod = "QRIS"
	PayKasbon   PaymentMethod = "KASBON"
)

type OrderStatus string

const (
	OrderCompleted OrderStatus = "COMPLETED"
	OrderCancelled OrderStatus = "CANCELLED"
)

// OrderItem represents a snapshot of an item inside an order (§4.2)
type OrderItem struct {
	ProductCode string `bson:"product_code" json:"kode_barang"`
	ProductName string `bson:"product_name" json:"nama_barang"`
	Qty         int64  `bson:"qty"          json:"qty"`
	UnitPrice   int64  `bson:"unit_price"   json:"harga_satuan"`
	Subtotal    int64  `bson:"subtotal"     json:"subtotal"`
}

// Order represents an order placed by a cooperative member (§4.2)
type Order struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderNo     string             `bson:"order_no"       json:"order_no"`
	MemberNIM   string             `bson:"member_nim"     json:"nim"`
	MemberName  string             `bson:"member_name"    json:"nama_anggota"`
	Items       []OrderItem        `bson:"items"          json:"items"`
	TotalAmount int64              `bson:"total_amount"   json:"total"`
	Payment     PaymentMethod      `bson:"payment_method" json:"metode_pembayaran"`
	Status      OrderStatus        `bson:"status"         json:"status"`
	CreatedAt   time.Time          `bson:"created_at"     json:"created_at"`
}

type OrderItemRequest struct {
	ProductCode string `json:"kode_barang"`
	Qty         int64  `json:"qty"`
}

type CreateOrderRequest struct {
	MemberNIM string             `json:"nim"`
	Payment   PaymentMethod      `json:"metode_pembayaran"`
	Items     []OrderItemRequest `json:"items"`
}

type OrderFilter struct {
	MemberNIM string
	From      *time.Time
	To        *time.Time
	Status    OrderStatus
	Page      int
	Limit     int
}

type OrderRepository interface {
	CreateWithTransaction(ctx context.Context, order *Order) error
	FindAll(ctx context.Context, filter OrderFilter) ([]Order, int64, error)
	FindByOrderNo(ctx context.Context, orderNo string) (*Order, error)
	CancelWithTransaction(ctx context.Context, orderNo string) error
}

type OrderService interface {
	Create(ctx context.Context, req *CreateOrderRequest) (*Order, error)
	List(ctx context.Context, filter OrderFilter) ([]Order, int64, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*Order, error)
	Cancel(ctx context.Context, orderNo string) error
}

