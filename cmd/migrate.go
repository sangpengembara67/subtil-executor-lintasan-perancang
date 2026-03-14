package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/shiestapoi/teletowa/database"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Define command line flags
	var (
		migrate = flag.Bool("migrate", false, "Run database migrations")
		seed = flag.Bool("seed", false, "Run database seeding")
		check = flag.Bool("check", false, "Check schema integrity")
		rollback = flag.Int("rollback", -1, "Rollback to specific migration version")
		clean = flag.Bool("clean", false, "Clean seeded data")
		verify = flag.Bool("verify", false, "Verify seed data")
		full = flag.Bool("full", false, "Run full migration and seeding")
		help = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Initialize database connection
	if err := database.InitDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.GetDB().Close()

	// Execute commands based on flags
	switch {
	case *full:
		runFullMigrationAndSeeding()
	case *migrate:
		runMigrations()
	case *seed:
		runSeeding()
	case *check:
		checkSchemaIntegrity()
	case *rollback >= 0:
		runRollback(*rollback)
	case *clean:
		cleanSeedData()
	case *verify:
		verifySeedData()
	default:
		fmt.Println("No command specified. Use -help for available options.")
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("Database Migration and Seeding Tool")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/migrate.go [options]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -migrate      Run database migrations")
	fmt.Println("  -seed         Run database seeding")
	fmt.Println("  -check        Check schema integrity")
	fmt.Println("  -rollback N   Rollback to migration version N")
	fmt.Println("  -clean        Clean seeded data")
	fmt.Println("  -verify       Verify seed data exists")
	fmt.Println("  -full         Run full migration and seeding")
	fmt.Println("  -help         Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  go run cmd/migrate.go -migrate")
	fmt.Println("  go run cmd/migrate.go -seed")
	fmt.Println("  go run cmd/migrate.go -full")
	fmt.Println("  go run cmd/migrate.go -rollback 0")
	fmt.Println("  go run cmd/migrate.go -check")
}

func runFullMigrationAndSeeding() {
	log.Println("=== Running Full Migration and Seeding ===")

	// Step 1: Run migrations
	log.Println("Step 1: Running migrations...")
	if err := database.RunMigrations(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Step 2: Check schema integrity
	log.Println("Step 2: Checking schema integrity...")
	if err := database.CheckSchemaIntegrity(); err != nil {
		log.Fatalf("Schema integrity check failed: %v", err)
	}

	// Step 3: Run seeding
	log.Println("Step 3: Running seeding...")
	if err := database.SeedWithSampleData(); err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}

	// Step 4: Verify seed data
	log.Println("Step 4: Verifying seed data...")
	if err := database.VerifySeedData(); err != nil {
		log.Fatalf("Seed data verification failed: %v", err)
	}

	log.Println("=== Full Migration and Seeding Completed Successfully ===")
}

func runMigrations() {
	log.Println("=== Running Database Migrations ===")
	if err := database.RunMigrations(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("=== Migrations Completed Successfully ===")
}

func runSeeding() {
	log.Println("=== Running Database Seeding ===")
	if err := database.SeedWithSampleData(); err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}
	log.Println("=== Seeding Completed Successfully ===")
}

func checkSchemaIntegrity() {
	log.Println("=== Checking Schema Integrity ===")
	if err := database.CheckSchemaIntegrity(); err != nil {
		log.Fatalf("Schema integrity check failed: %v", err)
	}
	log.Println("=== Schema Integrity Check Passed ===")
}

func runRollback(version int) {
	log.Printf("=== Rolling Back to Migration Version %d ===", version)
	if err := database.RollbackMigration(version); err != nil {
		log.Fatalf("Rollback failed: %v", err)
	}
	log.Printf("=== Rollback to Version %d Completed Successfully ===", version)
}

func cleanSeedData() {
	log.Println("=== Cleaning Seed Data ===")
	if err := database.CleanSeedData(); err != nil {
		log.Fatalf("Clean seed data failed: %v", err)
	}
	log.Println("=== Seed Data Cleaning Completed Successfully ===")
}

func verifySeedData() {
	log.Println("=== Verifying Seed Data ===")
	if err := database.VerifySeedData(); err != nil {
		log.Fatalf("Seed data verification failed: %v", err)
	}
	log.Println("=== Seed Data Verification Completed Successfully ===")
}