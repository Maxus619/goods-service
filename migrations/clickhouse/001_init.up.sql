CREATE TABLE IF NOT EXISTS goods_log (
    Id Int32,
    ProjectId Int32,
    Name String,
    Description String,
    Priority Int32,
    Removed Boolean,
    EventTime DateTime
) ENGINE = MergeTree()
ORDER BY (EventTime, Id);