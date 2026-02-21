package models

import (
	"time"

	"gorm.io/gorm"
)

type Mod struct {
	ID          string         `json:"id" gorm:"primaryKey;type:varchar(50)"`
	GameID      string         `json:"game_id" gorm:"type:varchar(50);not null;index"`
	Name        string         `json:"name" gorm:"type:varchar(100);not null"`
	Description string         `json:"description" gorm:"type:text"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}
