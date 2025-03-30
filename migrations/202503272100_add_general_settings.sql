ALTER TABLE tenants ADD COLUMN IF NOT EXISTS general_settings JSONB NOT NULL DEFAULT '{
  "titleLengthMin": 15,
  "titleLengthMax": 100,
  "descriptionLengthMin": 150,
  "descriptionLengthMax": 1000,
  "maxImagesPerPost": 3,
  "maxImagesPerComment": 2,
  "postLimits": {},
  "commentLimits": {},
  "postingDisabledFor": [],
  "commentingDisabledFor": [],
  "postingGloballyDisabled": false,
  "commentingGloballyDisabled": false
}';