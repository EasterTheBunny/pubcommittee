# bucket

Cloud Storage bucket with uniform access and enforced public-access prevention. Optional versioning and age-based deletion; object admin and viewer IAM members are inputs.

Inputs: `project`, `name`, `location`, `storage_class`, `versioning`, `delete_after_days`, `force_destroy`, `object_admins`, `object_viewers`. Output: `name`.
