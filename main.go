package kraken_db

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Add your model structs here (assuming they're in a separate package)
type PluginMetadata struct {
	ID                   uint                 `json:"id" gorm:"primaryKey"`
	Name                 string               `json:"name" gorm:"uniqueIndex"`
	Title                string               `json:"title"`
	Description          string               `json:"description"`
	ImageUrl             string               `json:"imageUrl"`
	VideoUrl             string               `json:"videoUrl"`
	TopPick              bool                 `json:"topPick"`
	Tier                 int                  `json:"tier"`
	PriceDetails         PluginPriceDetails   `json:"priceDetails" gorm:"foreignKey:PluginMetadataID"`
	ConfigurationOptions []PluginConfigOption `json:"configurationOptions" gorm:"foreignKey:PluginMetadataID"`
}

type PluginPriceDetails struct {
	ID               uint `json:"id" gorm:"primaryKey"`
	Month            int  `json:"month"`
	ThreeMonth       int  `json:"threeMonth"`
	Year             int  `json:"year"`
	PluginMetadataID uint `json:"pluginMetadataId"`
}

type PluginConfigOption struct {
	ID               uint     `json:"id" gorm:"primaryKey"`
	Name             string   `json:"name"`
	Section          string   `json:"section"`
	Description      string   `json:"description"`
	Type             string   `json:"type"`
	IsBool           bool     `json:"isBool"`
	Values           string   `json:"values"`
	ValuesSlice      []string `json:"values" gorm:"-"`
	PluginMetadataID uint     `json:"pluginMetadataId"`
}

type PluginPack struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	Name        string  `json:"name" gorm:"uniqueIndex"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ImageUrl    string  `json:"imageUrl"`
	Discount    float32 `json:"discount"`
	Active      bool    `json:"active"`
}

type PluginPackPriceDetails struct {
	ID               uint `json:"id" gorm:"primaryKey"`
	Month            int  `json:"month"`
	ThreeMonth       int  `json:"threeMonth"`
	Year             int  `json:"year"`
	PluginPackID     uint `json:"pluginPackId"`
	PluginMetadataID uint `json:"pluginMetadataId"`
}

type PluginPackItem struct {
	ID               uint `json:"id" gorm:"primaryKey"`
	PackID           uint `json:"packId"`
	PluginMetadataID uint `json:"pluginMetadataId"`
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

func main() {
	var (
		dbHost     = flag.String("db-host", "localhost", "Database host")
		dbPort     = flag.String("db-port", "3306", "Database port")
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

	// Setup logger
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()
	defer logger.Sync()

	// Connect to database
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
