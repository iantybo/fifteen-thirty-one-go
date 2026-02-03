-- Add user profile fields for name, address, email, and profile image
ALTER TABLE users ADD COLUMN full_name TEXT;
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN address TEXT;
ALTER TABLE users ADD COLUMN profile_image_path TEXT;
ALTER TABLE users ADD COLUMN mothers_maiden_name TEXT;
ALTER TABLE users ADD COLUMN billing_address TEXT;
ALTER TABLE users ADD COLUMN phone_number TEXT;
ALTER TABLE users ADD COLUMN date_of_birth TEXT;
-- Income field for player segmentation and targeted offers
ALTER TABLE users ADD COLUMN annual_income INTEGER;
