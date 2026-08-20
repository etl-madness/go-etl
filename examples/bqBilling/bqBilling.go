package bqbilling

// move example code to a different location to avoid confusion with the main.go file in the root of the project. This is a standalone example for fetching GCP billing data from BigQuery and outputting it as JSON.
// rename package to main to allow for building an executable
// and build.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// BillingExportRow maps the schema of GCP BigQuery Standard & Detailed Billing Exports.
type BillingExportRow struct {
	BillingAccountID       string               `bigquery:"billing_account_id" json:"billing_account_id"`
	Service                Service              `bigquery:"service" json:"service"`
	SKU                    SKU                  `bigquery:"sku" json:"sku"`
	UsageStartTime         time.Time            `bigquery:"usage_start_time" json:"usage_start_time"`
	UsageEndTime           time.Time            `bigquery:"usage_end_time" json:"usage_end_time"`
	Project                *Project             `bigquery:"project" json:"project,omitempty"`
	Labels                 []Label              `bigquery:"labels" json:"labels"`
	SystemLabels           []Label              `bigquery:"system_labels" json:"system_labels"`
	Location               Location             `bigquery:"location" json:"location"`
	ExportTime             time.Time            `bigquery:"export_time" json:"export_time"`
	Cost                   float64              `bigquery:"cost" json:"cost"`
	Currency               string               `bigquery:"currency" json:"currency"`
	CurrencyConversionRate float64              `bigquery:"currency_conversion_rate" json:"currency_conversion_rate"`
	Usage                  Usage                `bigquery:"usage" json:"usage"`
	Credits                []Credit             `bigquery:"credits" json:"credits"`
	Invoice                Invoice              `bigquery:"invoice" json:"invoice"`
	CostType               bigquery.NullString  `bigquery:"cost_type" json:"cost_type"`
	CostAtList             bigquery.NullFloat64 `bigquery:"cost_at_list" json:"cost_at_list"`
	Resource               *Resource            `bigquery:"resource" json:"resource,omitempty"`
	Tags                   []Tag                `bigquery:"tags" json:"tags"`
	AdjustmentInfo         *Adjustment          `bigquery:"adjustment_info" json:"adjustment_info,omitempty"`
}

type Service struct {
	ID          bigquery.NullString `bigquery:"id" json:"id"`
	Description bigquery.NullString `bigquery:"description" json:"description"`
}

type SKU struct {
	ID          bigquery.NullString `bigquery:"id" json:"id"`
	Description bigquery.NullString `bigquery:"description" json:"description"`
}

type Project struct {
	ID              bigquery.NullString `bigquery:"id" json:"id"`
	Name            bigquery.NullString `bigquery:"name" json:"name"`
	Number          bigquery.NullString `bigquery:"number" json:"number"`
	Labels          []Label             `bigquery:"labels" json:"labels"`
	AncestryNumbers bigquery.NullString `bigquery:"ancestry_numbers" json:"ancestry_numbers"`
}

type Label struct {
	Key   string `bigquery:"key" json:"key"`
	Value string `bigquery:"value" json:"value"`
}

type Location struct {
	Location bigquery.NullString `bigquery:"location" json:"location"`
	Country  bigquery.NullString `bigquery:"country" json:"country"`
	Region   bigquery.NullString `bigquery:"region" json:"region"`
	Zone     bigquery.NullString `bigquery:"zone" json:"zone"`
}

type Usage struct {
	Amount               float64             `bigquery:"amount" json:"amount"`
	Unit                 bigquery.NullString `bigquery:"unit" json:"unit"`
	AmountInPricingUnits float64             `bigquery:"amount_in_pricing_units" json:"amount_in_pricing_units"`
	PricingUnit          bigquery.NullString `bigquery:"pricing_unit" json:"pricing_unit"`
}

type Credit struct {
	Name     bigquery.NullString `bigquery:"name" json:"name"`
	Amount   float64             `bigquery:"amount" json:"amount"`
	FullName bigquery.NullString `bigquery:"full_name" json:"full_name"`
	ID       bigquery.NullString `bigquery:"id" json:"id"`
	Type     bigquery.NullString `bigquery:"type" json:"type"`
}

type Invoice struct {
	Month string `bigquery:"month" json:"month"`
}

type Resource struct {
	Name       bigquery.NullString `bigquery:"name" json:"name"`
	GlobalName bigquery.NullString `bigquery:"global_name" json:"global_name"`
}

type Tag struct {
	Key       string `bigquery:"key" json:"key"`
	Value     string `bigquery:"value" json:"value"`
	Inherited bool   `bigquery:"inherited" json:"inherited"`
	Namespace string `bigquery:"namespace" json:"namespace"`
}

type Adjustment struct {
	ID          bigquery.NullString `bigquery:"id" json:"id"`
	Description bigquery.NullString `bigquery:"description" json:"description"`
	Mode        bigquery.NullString `bigquery:"mode" json:"mode"`
	Type        bigquery.NullString `bigquery:"type" json:"type"`
}

func main() {
	projectID := flag.String("GCP_PROJECT_ID", "", "GCP Project ID")
	tablePath := flag.String("BQ_BILLING_TABLE", "", "BigQuery billing table (format: `project_id.dataset.table_name`)")
	credsFile := flag.String("GOOGLE_APPLICATION_CREDENTIALS", "", "Path to service account JSON key file")
	flag.Parse()
	ctx := context.Background()

	if *projectID == "" {
		log.Fatal("GCP_PROJECT_ID flag is required")
	}

	if *tablePath == "" {
		log.Fatal("BQ_BILLING_TABLE flag is required (format: `project_id.dataset.table_name`)")
	}

	if *credsFile == "" {
		log.Fatal("GOOGLE_APPLICATION_CREDENTIALS flag is required (path to service account JSON key file)")
	}

	client, err := bigquery.NewClient(
		ctx,
		*projectID,
		option.WithCredentialsFile(*credsFile),
	)
	if err != nil {
		log.Fatalf("Failed to create BigQuery client: %v", err)
	}
	defer client.Close()

	querySQL := fmt.Sprintf("SELECT * FROM `%s`", *tablePath)
	query := client.Query(querySQL)

	it, err := query.Read(ctx)
	if err != nil {
		log.Fatalf("Query execution failed: %v", err)
	}

	var records []BillingExportRow
	var grandTotal float64

	for {
		var row BillingExportRow
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Error iterating billing rows: %v", err)
		}

		records = append(records, row)
		grandTotal += row.Cost
	}

	// Direct stream encoder prevents stdout truncation on large output payloads
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(records); err != nil {
		log.Fatalf("Error encoding records array to JSON: %v", err)
	}
}
