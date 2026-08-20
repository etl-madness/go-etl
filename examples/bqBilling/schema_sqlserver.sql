CREATE TABLE
    dbo.GcpBillingExport (
        BillingExportID BIGINT IDENTITY (1, 1) PRIMARY KEY,
        BillingAccountID VARCHAR(32) NOT NULL,
        -- Service Info
        ServiceID VARCHAR(64) NULL,
        ServiceDescription NVARCHAR (255) NULL,
        -- SKU Info
        SkuID VARCHAR(64) NULL,
        SkuDescription NVARCHAR (255) NULL,
        -- Timestamps
        UsageStartTime DATETIMEOFFSET NOT NULL,
        UsageEndTime DATETIMEOFFSET NOT NULL,
        ExportTime DATETIMEOFFSET NOT NULL,
        -- Project Info
        ProjectID NVARCHAR (100) NULL,
        ProjectName NVARCHAR (100) NULL,
        ProjectNumber VARCHAR(32) NULL,
        ProjectAncestryNumbers NVARCHAR (255) NULL,
        ProjectLabels NVARCHAR (MAX) NULL,
        -- Location Info
        Location NVARCHAR (100) NULL,
        LocationCountry NVARCHAR (100) NULL,
        LocationRegion NVARCHAR (100) NULL,
        LocationZone NVARCHAR (100) NULL,
        -- Financials
        Cost DECIMAL(18, 6) NOT NULL,
        CostAtList DECIMAL(18, 6) NULL,
        CostType VARCHAR(50) NULL,
        Currency VARCHAR(10) NOT NULL,
        CurrencyConversionRate DECIMAL(18, 6) NOT NULL,
        -- Usage Data
        UsageAmount FLOAT NOT NULL,
        UsageUnit NVARCHAR (50) NULL,
        UsageAmountInPricingUnits FLOAT NOT NULL,
        UsagePricingUnit NVARCHAR (50) NULL,
        -- Invoice
        InvoiceMonth VARCHAR(6) NOT NULL,
        -- Resource Info
        ResourceName NVARCHAR (255) NULL,
        ResourceGlobalName NVARCHAR (1000) NULL,
        -- Adjustment Info
        AdjustmentID NVARCHAR (100) NULL,
        AdjustmentDescription NVARCHAR (255) NULL,
        AdjustmentMode NVARCHAR (50) NULL,
        AdjustmentType NVARCHAR (50) NULL,
        -- Dynamic Arrays (Stored as JSON)
        Labels NVARCHAR (MAX) NULL,
        SystemLabels NVARCHAR (MAX) NULL,
        Credits NVARCHAR (MAX) NULL,
        Tags NVARCHAR (MAX) NULL,
        -- JSON Validation Constraints
        CONSTRAINT CHK_GcpBilling_ProjectLabels CHECK (
            ProjectLabels IS NULL
            OR ISJSON (ProjectLabels) = 1
        ),
        CONSTRAINT CHK_GcpBilling_Labels CHECK (
            Labels IS NULL
            OR ISJSON (Labels) = 1
        ),
        CONSTRAINT CHK_GcpBilling_SystemLabels CHECK (
            SystemLabels IS NULL
            OR ISJSON (SystemLabels) = 1
        ),
        CONSTRAINT CHK_GcpBilling_Credits CHECK (
            Credits IS NULL
            OR ISJSON (Credits) = 1
        ),
        CONSTRAINT CHK_GcpBilling_Tags CHECK (
            Tags IS NULL
            OR ISJSON (Tags) = 1
        )
    );

-- Indexing for reporting and billing analytics queries
CREATE INDEX IX_GcpBillingExport_InvoiceMonth ON dbo.GcpBillingExport (InvoiceMonth, BillingAccountID);

CREATE INDEX IX_GcpBillingExport_UsageTime ON dbo.GcpBillingExport (UsageStartTime, UsageEndTime);

CREATE INDEX IX_GcpBillingExport_Project_Service ON dbo.GcpBillingExport (ProjectID, ServiceID);