package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"os"
	"time"
)

func main() {
	var (
		dbHost     = flag.String("db-host", "localhost", "Database host")
		dbPort     = flag.String("db-port", "30306", "Database port") // 30306 for Kube node port
		dbUser     = flag.String("db-user", "root", "Database user")
		dbPassword = flag.String("db-password", "", "Database password")
		dbName     = flag.String("db-name", "", "Database name")
		pluginFile = flag.String("plugin-file", "./data/plugin_metadata.json", "Path to plugin metadata JSON file")
		packFile   = flag.String("pack-file", "./data/plugin_packs.json", "Path to plugin pack JSON file")
		dryRun     = flag.Bool("dry-run", false, "Run without making changes")
	)
	flag.Parse()

	if *dbName == "" {
		log.Fatal("Database name is required")
	}

	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	defer logger.Sync()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		*dbUser, *dbPassword, *dbHost, *dbPort, *dbName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	sugar.Infof("Connected to database: %s", *dbName)

	if *dryRun {
		sugar.Info("DRY RUN MODE - No changes will be made")
		// In dry run, you could validate JSON and check what would be changed
		if *pluginFile != "" {
			sugar.Infof("Would import plugin metadata from: %s", *pluginFile)
		}
		if *packFile != "" {
			sugar.Infof("Would import plugin packs from: %s", *packFile)
		}
		return
	}

	// Import plugin metadata if file provided
	if *pluginFile != "" {
		sugar.Infof("Importing plugin metadata from: %s", *pluginFile)
		if err := ImportOrUpdatePluginMetadata(*pluginFile, db, sugar); err != nil {
			log.Fatal("Failed to import plugin metadata:", err)
		}
		sugar.Info("Plugin metadata import completed successfully")
	}

	// Import plugin packs if file provided
	if *packFile != "" {
		sugar.Infof("Importing plugin packs from: %s", *packFile)
		if err := ImportOrUpdatePluginPacks(*packFile, db, sugar); err != nil {
			log.Fatal("Failed to import plugin packs:", err)
		}
		sugar.Info("Plugin packs import completed successfully")
	}

	if *pluginFile == "" && *packFile == "" {
		sugar.Info("No files specified. Use -plugin-file or -pack-file flags")
	}
}

func ImportOrUpdatePluginMetadata(jsonFilePath string, db *gorm.DB, log *zap.SugaredLogger) error {
	jsonData, err := os.ReadFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	var pluginMetadataList []PluginMetadata
	if err := json.Unmarshal(jsonData, &pluginMetadataList); err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	for i := range pluginMetadataList {
		plugin := &pluginMetadataList[i]

		// Check if the plugin already exists by Name (which should be unique)
		var existingPlugin PluginMetadata
		log.Debugf("finding plugin: %s", plugin.Name)
		result := tx.Where("name = ?", plugin.Name).First(&existingPlugin)

		if result.Error == nil {
			log.Debugf("plugin already exists: %s", plugin.Name)
			continue
		}
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Plugin doesn't exist, create it
			priceDetails := plugin.PriceDetails
			configOptions := plugin.ConfigurationOptions
			plugin.ConfigurationOptions = nil

			if err := tx.Create(plugin).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create plugin metadata: %w", err)
			}

			priceDetails.PluginMetadataID = plugin.ID
			if err := tx.Create(&priceDetails).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create price details: %w", err)
			}

			for j := range configOptions {
				if len(configOptions[j].ValuesSlice) > 0 {
					valuesData, err := json.Marshal(configOptions[j].ValuesSlice)
					if err != nil {
						tx.Rollback()
						return fmt.Errorf("failed to marshal values: %w", err)
					}
					configOptions[j].Values = string(valuesData)
				}

				configOptions[j].PluginMetadataID = plugin.ID
				if err := tx.Create(&configOptions[j]).Error; err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to create config option: %w", err)
				}
			}
		} else {
			tx.Rollback()
			return fmt.Errorf("error checking for existing plugin: %v", result.Error)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func ImportOrUpdatePluginPacks(jsonFilePath string, db *gorm.DB, log *zap.SugaredLogger) error {
	// Read the JSON file
	jsonData, err := os.ReadFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Unmarshal the JSON data
	var pluginPackInputs []PluginPackInput
	if err := json.Unmarshal(jsonData, &pluginPackInputs); err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	// Begin a transaction
	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	for _, packInput := range pluginPackInputs {
		// Check if the pack already exists
		var existingPack PluginPack
		result := tx.Where("name = ?", packInput.Name).First(&existingPack)

		if result.Error == nil {
			log.Debugf("pack already exists: %s", packInput.Name)
			continue
		}

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new pack
			pack := PluginPack{
				Name:        packInput.Name,
				Title:       packInput.Title,
				Description: packInput.Description,
				ImageUrl:    packInput.ImageUrl,
				Discount:    packInput.Discount,
				Active:      packInput.Active,
			}

			// Create the plugin pack
			if err := tx.Create(&pack).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create plugin pack: %w", err)
			}

			// Create the price details - explicitly set PluginMetadataID to zero/NULL
			priceDetails := PluginPackPriceDetails{
				Month:        packInput.PriceDetails.Month,
				ThreeMonth:   packInput.PriceDetails.ThreeMonth,
				Year:         packInput.PriceDetails.Year,
				PluginPackID: pack.ID,
			}

			// Use SQL that doesn't include PluginMetadataID in the INSERT statement
			if err := tx.Omit("PluginMetadataID").Create(&priceDetails).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create price details: %w", err)
			}

			// Link the plugins to the pack
			for _, pluginName := range packInput.Plugins {
				// Find the plugin metadata by name
				var pluginMetadata PluginMetadata
				if err := tx.Where("name = ?", pluginName).First(&pluginMetadata).Error; err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to find plugin metadata '%s': %w", pluginName, err)
				}

				// Create the pack item
				packItem := PluginPackItem{
					PackID:           pack.ID,
					PluginMetadataID: pluginMetadata.ID,
				}
				if err := tx.Create(&packItem).Error; err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to create plugin pack item: %w", err)
				}
			}
		} else if result.Error != nil {
			tx.Rollback()
			return fmt.Errorf("error checking for existing plugin pack: %w", result.Error)
		} else {
			// Pack exists, update it
			existingPack.Title = packInput.Title
			existingPack.Description = packInput.Description
			existingPack.ImageUrl = packInput.ImageUrl
			existingPack.Discount = packInput.Discount
			existingPack.Active = packInput.Active

			if err := tx.Save(&existingPack).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update plugin pack: %w", err)
			}

			// Update price details - make sure we're not updating the PluginMetadataID
			if err := tx.Model(&PluginPackPriceDetails{}).
				Where("plugin_pack_id = ?", existingPack.ID).
				Updates(map[string]interface{}{
					"month":       packInput.PriceDetails.Month,
					"three_month": packInput.PriceDetails.ThreeMonth,
					"year":        packInput.PriceDetails.Year,
				}).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update price details: %w", err)
			}

			// Delete existing pack items
			if err := tx.Where("pack_id = ?", existingPack.ID).Delete(&PluginPackItem{}).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to delete existing plugin pack items: %w", err)
			}

			// Re-add the plugins
			for _, pluginName := range packInput.Plugins {
				// Find the plugin metadata by name
				var pluginMetadata PluginMetadata
				if err := tx.Where("name = ?", pluginName).First(&pluginMetadata).Error; err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to find plugin metadata '%s': %w", pluginName, err)
				}

				// Create the pack item
				packItem := PluginPackItem{
					PackID:           existingPack.ID,
					PluginMetadataID: pluginMetadata.ID,
				}
				if err := tx.Create(&packItem).Error; err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to create plugin pack item: %w", err)
				}
			}
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

type PluginPackInput struct {
	Name         string                 `json:"name"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	ImageUrl     string                 `json:"imageUrl"`
	Discount     float32                `json:"discount"`
	Active       bool                   `json:"active"`
	Plugins      []string               `json:"plugins"`
	PriceDetails PluginPackPriceDetails `json:"priceDetails"`
}

// PluginPack represents a collection of plugins sold together
type PluginPack struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"column:name;not null" json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ImageUrl    string         `json:"imageUrl"`
	Discount    float32        `gorm:"column:discount;default:0" json:"discount"` // Percentage discount when buying the pack
	Active      bool           `gorm:"column:active;default:true" json:"active"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`

	// Relations
	Items        []PluginPackItem       `gorm:"foreignKey:PackID" json:"items"`
	PriceDetails PluginPackPriceDetails `gorm:"foreignKey:PluginPackID" json:"priceDetails"`
}

// PluginPackItem represents a plugin that belongs to a pack
type PluginPackItem struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	PackID           uint           `gorm:"column:pack_id;index" json:"packId"`
	PluginMetadataID uint           `gorm:"column:plugin_metadata_id;index" json:"pluginMetadataId"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`

	// Relations
	PluginMetadata PluginMetadata `gorm:"foreignKey:PluginMetadataID" json:"pluginMetadata"`
}

type PluginMetadataPriceDetails struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	Month      int  `json:"month"`
	ThreeMonth int  `json:"threeMonth"`
	Year       int  `json:"year"`

	// In a JSON serialized response additional metadata about the sale price for a plugin can be included optionally in the response.
	// If a sale is not happening for a plugin these fields can be safely ignored and will not be returned in the response.
	SaleMonth        int            `gorm:"-" json:"saleMonth,omitempty"`
	SaleThreeMonth   int            `gorm:"-" json:"saleThreeMonth,omitempty"`
	SaleYear         int            `gorm:"-" json:"saleYear,omitempty"`
	PluginMetadataID uint           `gorm:"column:plugin_metadata_id;index;not null" json:"pluginMetadataId"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

func (p PluginMetadataPriceDetails) TableName() string {
	return "plugin_metadata_price_details"
}

// PluginPackPriceDetails specifically for pack pricing
type PluginPackPriceDetails struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Month        int            `json:"month"`
	ThreeMonth   int            `json:"threeMonth"`
	Year         int            `json:"year"`
	PluginPackID uint           `gorm:"column:plugin_pack_id;index;not null" json:"pluginPackId"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

func (p PluginPackPriceDetails) TableName() string {
	return "plugin_pack_price_details"
}

type CognitoCredentials struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"column:user_id;index" json:"userId"`
	RefreshToken    string         `gorm:"column:refresh_token;type:LONGTEXT" json:"refreshToken,omitempty"`
	TokenExpiration int32          `gorm:"column:token_expiration" json:"tokenExpirationSeconds,omitempty"`
	AccessToken     string         `gorm:"column:access_token;type:LONGTEXT" json:"accessToken,omitempty"`
	IdToken         string         `gorm:"column:id_token;type:LONGTEXT" json:"idToken,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

// HardwareID represents a hardware identifier associated with a user
type HardwareID struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Value     string         `gorm:"uniqueIndex;not null"`
	UserID    uint           `gorm:"column:user_id;index" json:"userId"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

func (HardwareID) TableName() string {
	return "hardware_ids"
}

type PluginMetadata struct {
	ID                   uint                       `gorm:"primaryKey" json:"id"`
	Name                 string                     `gorm:"uniqueIndex" json:"name"`
	Title                string                     `json:"title"`
	Description          string                     `json:"description"`
	ImageUrl             string                     `json:"imageUrl"`
	VideoUrl             string                     `json:"videoUrl"`
	TopPick              bool                       `json:"topPick"`
	IsInBeta             bool                       `json:"isInBeta"`
	Version              string                     `gorm:"-" json:"version"`
	SaleDiscount         float32                    `gorm:"-" json:"saleDiscount"` // Sale discounts are pulled from the db but included only in API responses not on actual rows in db.
	ConfigurationOptions []PluginConfig             `gorm:"foreignKey:PluginMetadataID" json:"configurationOptions"`
	PriceDetails         PluginMetadataPriceDetails `gorm:"foreignKey:PluginMetadataID" json:"priceDetails"`
	Tier                 int                        `json:"tier"`
}

type PluginConfig struct {
	ID               uint     `gorm:"primaryKey" json:"id"`
	Name             string   `json:"name"`
	Section          string   `json:"section"`
	Description      string   `json:"description"`
	Type             string   `json:"type"`
	IsBool           bool     `json:"isBool"`
	Values           string   `gorm:"type:text" json:"-"`
	ValuesSlice      []string `gorm:"-" json:"values"`
	PluginMetadataID uint     `json:"pluginMetadataId"`
}
