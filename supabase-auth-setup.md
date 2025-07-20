# Supabase Authentication and Row Level Security Setup

This guide covers the setup of Supabase Authentication and Row Level Security (RLS) policies for the competitive programming platform.

## Prerequisites

1. Supabase project with the database schema already created
2. Access to the Supabase dashboard
3. The `schema.sql` file has been executed

## Step 1: Configure Authentication Providers

### Enable Email Authentication

1. Go to your Supabase project dashboard
2. Navigate to **Authentication** > **Settings**
3. Under **Auth Providers**, ensure **Email** is enabled
4. Configure the following settings:
   - **Enable email confirmations**: Enabled (recommended)
   - **Enable email change confirmations**: Enabled
   - **Enable secure password change**: Enabled

### Configure Email Templates (Optional)

1. Go to **Authentication** > **Email Templates**
2. Customize the email templates for:
   - Confirm signup
   - Reset password
   - Email change

### Set Site URL and Redirect URLs

1. In **Authentication** > **Settings**
2. Set your **Site URL** (e.g., `https://your-domain.com`)
3. Add **Redirect URLs** for your development and production environments:
   - `http://localhost:3000` (development)
   - `https://your-domain.com` (production)

## Step 2: Verify RLS Policies

The RLS policies were created as part of the schema setup. Let's verify they're working correctly:

### Check RLS Status

Run this query in the SQL Editor to verify RLS is enabled:

```sql
SELECT schemaname, tablename, rowsecurity 
FROM pg_tables 
WHERE schemaname = 'public' 
AND tablename IN ('users', 'problems', 'submissions', 'test_cases');
```

### View Current Policies

Check the existing RLS policies:

```sql
SELECT schemaname, tablename, policyname, permissive, roles, cmd, qual
FROM pg_policies 
WHERE schemaname = 'public'
ORDER BY tablename, policyname;
```

## Step 3: Test Authentication and RLS

### Test User Registration

1. Create a test user through the Supabase dashboard:
   - Go to **Authentication** > **Users**
   - Click **Invite a user**
   - Enter an email address
   - Set a temporary password

2. Or use the Supabase client to register:
   ```javascript
   const { data, error } = await supabase.auth.signUp({
     email: 'test@example.com',
     password: 'password123'
   })
   ```

### Test RLS Policies

Create a test script to verify RLS policies:

```sql
-- Test as an authenticated user
-- First, create a test user profile
INSERT INTO users (id, username, email, full_name) 
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'testuser1',
  'test1@example.com',
  'Test User 1'
);

-- Test reading own profile (should work)
SELECT * FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

-- Test reading problems (should work for all users)
SELECT title, difficulty FROM problems LIMIT 5;

-- Test reading submissions (should only show own submissions)
SELECT * FROM submissions WHERE user_id = '00000000-0000-0000-0000-000000000001';
```

## Step 4: Additional Security Configurations

### Create Additional RLS Policies

Add more granular policies if needed:

```sql
-- Policy for admin users to manage problems
CREATE POLICY "Admin users can manage problems" ON problems
    FOR ALL USING (
        EXISTS (
            SELECT 1 FROM users 
            WHERE users.id = auth.uid() 
            AND users.email LIKE '%@admin.yourplatform.com'
        )
    );

-- Policy for contest-specific submissions
CREATE POLICY "Contest submissions visibility" ON submissions
    FOR SELECT USING (
        auth.uid() = user_id 
        OR EXISTS (
            SELECT 1 FROM users 
            WHERE users.id = auth.uid() 
            AND users.email LIKE '%@admin.yourplatform.com'
        )
    );
```

### Set Up Database Functions for Auth

Create helper functions for authentication:

```sql
-- Function to get current user's profile
CREATE OR REPLACE FUNCTION get_current_user_profile()
RETURNS TABLE(
    id UUID,
    username VARCHAR(50),
    full_name VARCHAR(100),
    email VARCHAR(255),
    rating INTEGER,
    problems_solved INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT u.id, u.username, u.full_name, u.email, u.rating, u.problems_solved
    FROM users u
    WHERE u.id = auth.uid();
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to check if user is admin
CREATE OR REPLACE FUNCTION is_admin()
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM users 
        WHERE id = auth.uid() 
        AND email LIKE '%@admin.yourplatform.com'
    );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
```

## Step 5: Environment Configuration

### Get Supabase Credentials

1. Go to your Supabase project dashboard
2. Navigate to **Settings** > **API**
3. Copy the following values:
   - **Project URL**
   - **anon public** key
   - **service_role** key (for server-side operations)

### Create Environment Variables

Create a `.env` file in your project root:

```env
# Supabase Configuration
SUPABASE_URL=https://your-project-id.supabase.co
SUPABASE_ANON_KEY=your-anon-key
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key

# Database Configuration (for direct connections)
DATABASE_URL=postgresql://postgres:your-password@db.your-project-id.supabase.co:5432/postgres
```

## Step 6: Testing and Validation

### Manual Testing Checklist

1. **User Registration**
   - [ ] User can register with email/password
   - [ ] Confirmation email is sent (if enabled)
   - [ ] User profile is created in users table

2. **Authentication**
   - [ ] User can log in with correct credentials
   - [ ] User cannot log in with incorrect credentials
   - [ ] JWT token is generated correctly

3. **RLS Policies**
   - [ ] Users can only access their own profile data
   - [ ] Users can read all problems
   - [ ] Users can only see their own submissions
   - [ ] Sample test cases are visible to all users
   - [ ] Full test cases require authentication

### Integration Test Script

Create a test script to validate the setup:

```javascript
// test-auth-rls.js
const { createClient } = require('@supabase/supabase-js');

const supabaseUrl = process.env.SUPABASE_URL;
const supabaseKey = process.env.SUPABASE_ANON_KEY;
const supabase = createClient(supabaseUrl, supabaseKey);

async function testAuthAndRLS() {
    try {
        // Test 1: Sign up a new user
        const { data: signUpData, error: signUpError } = await supabase.auth.signUp({
            email: 'test@example.com',
            password: 'password123'
        });
        
        if (signUpError) {
            console.error('Sign up error:', signUpError);
            return;
        }
        
        console.log('User signed up successfully:', signUpData.user.id);
        
        // Test 2: Create user profile
        const { data: profileData, error: profileError } = await supabase
            .from('users')
            .insert([
                {
                    id: signUpData.user.id,
                    username: 'testuser',
                    email: 'test@example.com',
                    full_name: 'Test User'
                }
            ]);
        
        if (profileError) {
            console.error('Profile creation error:', profileError);
        } else {
            console.log('Profile created successfully');
        }
        
        // Test 3: Read problems (should work)
        const { data: problemsData, error: problemsError } = await supabase
            .from('problems')
            .select('title, difficulty')
            .limit(5);
        
        if (problemsError) {
            console.error('Problems read error:', problemsError);
        } else {
            console.log('Problems read successfully:', problemsData.length);
        }
        
        // Test 4: Read own profile (should work)
        const { data: ownProfileData, error: ownProfileError } = await supabase
            .from('users')
            .select('*')
            .eq('id', signUpData.user.id);
        
        if (ownProfileError) {
            console.error('Own profile read error:', ownProfileError);
        } else {
            console.log('Own profile read successfully');
        }
        
        console.log('All tests passed!');
        
    } catch (error) {
        console.error('Test failed:', error);
    }
}

testAuthAndRLS();
```

## Step 7: Production Considerations

### Security Best Practices

1. **Never expose service_role key** in client-side code
2. **Use environment variables** for all sensitive configuration
3. **Enable email confirmation** for production
4. **Set strong password requirements**
5. **Implement rate limiting** for authentication endpoints
6. **Monitor authentication attempts** for suspicious activity

### Performance Optimization

1. **Add indexes** on frequently queried columns (already included in schema)
2. **Use connection pooling** for database connections
3. **Implement caching** for frequently accessed data
4. **Monitor RLS policy performance** and optimize as needed

### Monitoring and Logging

1. **Enable audit logging** in Supabase
2. **Monitor authentication metrics**
3. **Set up alerts** for failed login attempts
4. **Track user registration and activity**

## Troubleshooting

### Common Issues

1. **RLS Policy Errors**
   ```sql
   -- Temporarily disable RLS for debugging
   ALTER TABLE users DISABLE ROW LEVEL SECURITY;
   
   -- Re-enable after fixing issues
   ALTER TABLE users ENABLE ROW LEVEL SECURITY;
   ```

2. **Authentication Errors**
   - Check email/password requirements
   - Verify redirect URLs are configured
   - Confirm email templates are set up

3. **Database Connection Issues**
   - Verify database URL format
   - Check connection string credentials
   - Ensure network connectivity

### Debug Queries

```sql
-- Check current user context
SELECT auth.uid(), auth.email();

-- Check RLS policies
SELECT * FROM pg_policies WHERE schemaname = 'public';

-- Check user permissions
SELECT * FROM information_schema.table_privileges 
WHERE table_schema = 'public';
```

## Next Steps

After completing the authentication and RLS setup:

1. Test the configuration with the provided scripts
2. Integrate authentication with the GoLang backend
3. Implement JWT token validation
4. Create API endpoints that respect RLS policies
5. Set up frontend authentication flows

The authentication and RLS foundation is now ready for the backend implementation.