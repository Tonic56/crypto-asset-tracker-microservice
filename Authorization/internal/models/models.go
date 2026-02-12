
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (user *User) BeforeCreate(tx *gorm.DB) (err error) {
	user.ID = uuid.New()
	return
}

type User struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;" json:"id"`
	Name     string    `gorm:"unique" json:"name"`
	Password string    `json:"password"`
}

type Session struct {
	ID           uint      
	UserID       uuid.UUID `gorm:"type:uuid"`
	User         User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;"`
	RefreshToken string    `gorm:"unique"`
	ExpiresAt    time.Time
}