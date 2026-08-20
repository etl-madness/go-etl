DECLARE @J NVARCHAR(MAX) = N'<paste_json_here>';

SELECT 
    -- Root Identifiers
    BillingAccountID          = billing_account_id,
    
    -- Service Info
    ServiceID                 = service_id,
    ServiceDescription       = service_description,
    
    -- SKU Info
    SkuID                     = sku_id,
    SkuDescription           = sku_description,
    
    -- Timestamps
    UsageStartTime            = usage_start_time,
    UsageEndTime              = usage_end_time,
    ExportTime                = export_time,
    
    -- Project Info
    ProjectID                 = project_id,
    ProjectName               = project_name,
    ProjectNumber             = project_number,
    ProjectAncestryNumbers   = project_ancestry_numbers,
    ProjectLabels             = project_labels,
    
    -- Location Info
    Location                  = location_location,
    LocationCountry           = location_country,
    LocationRegion            = location_region,
    LocationZone              = location_zone,
    
    -- Financials
    Cost                      = cost,
    CostAtList                = cost_at_list,
    CostType                  = cost_type,
    Currency                  = currency,
    CurrencyConversionRate    = currency_conversion_rate,
    
    -- Usage Data
    UsageAmount               = usage_amount,
    UsageUnit                 = usage_unit,
    UsageAmountInPricingUnits = usage_amount_in_pricing_units,
    UsagePricingUnit          = usage_pricing_unit,
    
    -- Invoice
    InvoiceMonth              = invoice_month,
    
    -- Resource Info
    ResourceName              = resource_name,
    ResourceGlobalName        = resource_global_name,
    
    -- Adjustment Info
    AdjustmentID              = adjustment_id,
    AdjustmentDescription     = adjustment_description,
    AdjustmentMode            = adjustment_mode,
    AdjustmentType            = adjustment_type,
    
    -- Nested Array Fields (Preserved as Raw JSON Strings)
    Labels                    = labels,
    SystemLabels              = system_labels,
    Credits                   = credits,
    Tags                      = tags
FROM OPENJSON(@J)
WITH (
    -- Root & Scalar Paths
    billing_account_id            VARCHAR(32)           '$.billing_account_id',
    usage_start_time              DATETIMEOFFSET        '$.usage_start_time',
    usage_end_time                DATETIMEOFFSET        '$.usage_end_time',
    export_time                   DATETIMEOFFSET        '$.export_time',
    cost                          DECIMAL(18,6)         '$.cost',
    currency                      VARCHAR(10)           '$.currency',
    currency_conversion_rate      DECIMAL(18,6)         '$.currency_conversion_rate',
    cost_type                     VARCHAR(50)           '$.cost_type',
    cost_at_list                  DECIMAL(18,6)         '$.cost_at_list',

    -- Nested Object Paths
    service_id                    VARCHAR(64)           '$.service.id',
    service_description           NVARCHAR(255)         '$.service.description',
    sku_id                        VARCHAR(64)           '$.sku.id',
    sku_description               NVARCHAR(255)         '$.sku.description',
    project_id                    NVARCHAR(100)         '$.project.id',
    project_name                  NVARCHAR(100)         '$.project.name',
    project_number                VARCHAR(32)           '$.project.number',
    project_ancestry_numbers      NVARCHAR(255)         '$.project.ancestry_numbers',
    location_location             NVARCHAR(100)         '$.location.location',
    location_country              NVARCHAR(100)         '$.location.country',
    location_region               NVARCHAR(100)         '$.location.region',
    location_zone                 NVARCHAR(100)         '$.location.zone',
    usage_amount                  FLOAT                 '$.usage.amount',
    usage_unit                    NVARCHAR(50)          '$.usage.unit',
    usage_amount_in_pricing_units FLOAT                 '$.usage.amount_in_pricing_units',
    usage_pricing_unit            NVARCHAR(50)          '$.usage.pricing_unit',
    invoice_month                 VARCHAR(6)            '$.invoice.month',
    resource_name                 NVARCHAR(255)         '$.resource.name',
    resource_global_name          NVARCHAR(1000)        '$.resource.global_name',
    adjustment_id                 NVARCHAR(100)         '$.adjustment_info.id',
    adjustment_description        NVARCHAR(255)         '$.adjustment_info.description',
    adjustment_mode               NVARCHAR(50)          '$.adjustment_info.mode',
    adjustment_type               NVARCHAR(50)          '$.adjustment_info.type',

    -- Array Paths (AS JSON preserves array structures)
    project_labels                NVARCHAR(MAX)         '$.project.labels' AS JSON,
    labels                        NVARCHAR(MAX)         '$.labels'         AS JSON,
    system_labels                 NVARCHAR(MAX)         '$.system_labels'  AS JSON,
    credits                       NVARCHAR(MAX)         '$.credits'        AS JSON,
    tags                          NVARCHAR(MAX)         '$.tags'           AS JSON
);