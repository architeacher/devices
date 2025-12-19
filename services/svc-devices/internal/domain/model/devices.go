package model

import (
	"time"

	"github.com/google/uuid"
)

type DeviceID struct {
	uuid.UUID
}

func NewDeviceID() DeviceID {
	return DeviceID{UUID: uuid.Must(uuid.NewV7())}
}

func ParseDeviceID(s string) (DeviceID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return DeviceID{}, err
	}

	return DeviceID{UUID: id}, nil
}

func (d DeviceID) String() string {
	return d.UUID.String()
}

func (d DeviceID) IsZero() bool {
	return d.UUID == uuid.Nil
}

type Device struct {
	ID        DeviceID
	Name      string
	Brand     string
	State     State
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewDevice(name, brand string, state State) *Device {
	now := time.Now().UTC()

	return &Device{
		ID:        NewDeviceID(),
		Name:      name,
		Brand:     brand,
		State:     state,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (d *Device) CanUpdateNameAndBrand() bool {
	return d.State != StateInUse
}

func (d *Device) CanDelete() bool {
	return d.State != StateInUse
}

func (d *Device) Update(name, brand string, state State) error {
	if !d.CanUpdateNameAndBrand() && (name != d.Name || brand != d.Brand) {
		return ErrCannotUpdateInUseDevice
	}

	d.Name = name
	d.Brand = brand
	d.State = state
	d.UpdatedAt = time.Now().UTC()

	return nil
}

func (d *Device) Patch(updates map[string]any) error {
	if !d.CanUpdateNameAndBrand() {
		if name, ok := updates["name"].(string); ok && name != d.Name {
			return ErrCannotUpdateInUseDevice
		}

		if brand, ok := updates["brand"].(string); ok && brand != d.Brand {
			return ErrCannotUpdateInUseDevice
		}
	}

	if name, ok := updates["name"].(string); ok {
		d.Name = name
	}

	if brand, ok := updates["brand"].(string); ok {
		d.Brand = brand
	}

	if stateStr, ok := updates["state"].(string); ok {
		state, err := ParseState(stateStr)
		if err != nil {
			return err
		}

		d.State = state
	}

	d.UpdatedAt = time.Now().UTC()

	return nil
}

type DeviceFilter struct {
	Brand *string
	State *State
	Page  uint
	Size  uint
	Sort  string
}

func DefaultDeviceFilter() DeviceFilter {
	return DeviceFilter{
		Page: 1,
		Size: 20,
		Sort: "-createdAt",
	}
}

type Pagination struct {
	Page        uint
	Size        uint
	TotalItems  uint
	TotalPages  uint
	HasNext     bool
	HasPrevious bool
}

type DeviceList struct {
	Devices    []*Device
	Pagination Pagination
	Filters    DeviceFilter
}
