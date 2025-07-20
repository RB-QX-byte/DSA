# Database Setup Instructions

This document explains how to set up the PostgreSQL database schema for the competitive programming platform using Supabase.

## Prerequisites

1. A Supabase account and project
2. Access to the Supabase SQL Editor or a PostgreSQL client connected to your Supabase database

## Schema Overview

The database schema includes the following main tables:

### Core Tables

1. **users** - Extended user profiles linked to Supabase auth.users
   - Stores username, rating, problems solved, contests participated
   - Indexes on username, rating, and creation date for fast lookups

2. **problems** - Competitive programming problems
   - Stores problem title, description, difficulty, time/memory limits
   - Includes tags array for categorization
   - Tracks acceptance rate and submission statistics

3. **test_cases** - Test cases for each problem
   - Stores input/output pairs for problem validation
   - Distinguishes between sample and hidden test cases

4. **submissions** - User code submissions
   - Tracks submission verdict, execution time, memory usage
   - Links users to problems with submission results

### Key Features

- **UUID Primary Keys**: All tables use UUID for better scalability and security
- **Optimized Indexes**: Strategic indexes on frequently queried columns
- **Automatic Timestamps**: Auto-updating created_at and updated_at fields
- **Statistics Tracking**: Automatic updates to problem and user statistics
- **Row Level Security**: Prepared RLS policies for data isolation

## Setup Instructions

### Step 1: Connect to Supabase

1. Go to your Supabase project dashboard
2. Navigate to the SQL Editor
3. Create a new query

### Step 2: Execute the Schema

1. Copy the contents of `schema.sql`
2. Paste into the SQL Editor
3. Execute the query

### Step 3: Verify Installation

Run the following queries to verify the schema was created correctly:

```sql
-- Check tables were created
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public' 
AND table_name IN ('users', 'problems', 'test_cases', 'submissions');

-- Check indexes were created
SELECT schemaname, tablename, indexname 
FROM pg_indexes 
WHERE tablename IN ('users', 'problems', 'test_cases', 'submissions');

-- Check sample data was inserted
SELECT title, difficulty FROM problems;
```

### Step 4: Configure Authentication

The schema assumes Supabase Auth is enabled. Ensure:

1. Auth is enabled in your Supabase project
2. Email authentication is configured
3. User registration is allowed

## Schema Details

### Users Table

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY REFERENCES auth.users(id),
    username VARCHAR(50) UNIQUE NOT NULL,
    full_name VARCHAR(100),
    email VARCHAR(255) UNIQUE NOT NULL,
    rating INTEGER DEFAULT 1200,
    max_rating INTEGER DEFAULT 1200,
    problems_solved INTEGER DEFAULT 0,
    contests_participated INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Problems Table

```sql
CREATE TABLE problems (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT NOT NULL,
    difficulty INTEGER NOT NULL CHECK (difficulty >= 800 AND difficulty <= 3500),
    time_limit INTEGER NOT NULL DEFAULT 1000,
    memory_limit INTEGER NOT NULL DEFAULT 256,
    tags TEXT[],
    acceptance_rate DECIMAL(5,2) DEFAULT 0.00,
    total_submissions INTEGER DEFAULT 0,
    accepted_submissions INTEGER DEFAULT 0,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

## Row Level Security (RLS)

The schema includes RLS policies for data security:

- **Users**: Can only view/update their own profiles
- **Problems**: Public read access, authenticated write access
- **Submissions**: Users can only view their own submissions
- **Test Cases**: Sample cases are public, all cases visible to authenticated users

## Sample Data

The schema includes sample problems for testing:

1. **Two Sum** (Difficulty: 800)
2. **Maximum Subarray** (Difficulty: 1200)
3. **Binary Tree Inorder Traversal** (Difficulty: 1000)

Each sample problem includes at least one test case for validation.

## Next Steps

After setting up the database schema:

1. Configure Supabase Auth settings
2. Set up the GoLang backend to connect to the database
3. Implement the User Management service
4. Implement the Problem Management service
5. Test the schema with sample data and queries

## Troubleshooting

### Common Issues

1. **Extension Error**: If you get an error about uuid-ossp extension, ensure it's enabled in your Supabase project extensions.

2. **RLS Policies**: If you encounter RLS policy errors, you can temporarily disable RLS on specific tables:
   ```sql
   ALTER TABLE table_name DISABLE ROW LEVEL SECURITY;
   ```

3. **Index Creation**: If indexes fail to create, check for duplicate index names or conflicting constraints.

## Performance Considerations

- **Indexes**: The schema includes optimized indexes for common query patterns
- **Triggers**: Automatic statistics updates may impact write performance at scale
- **Partitioning**: Consider partitioning the submissions table by date for large datasets

## Security Notes

- All sensitive operations require authentication
- Test cases include both sample (public) and hidden test cases
- User data is isolated through RLS policies
- UUIDs prevent enumeration attacks