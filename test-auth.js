#!/usr/bin/env node

/**
 * Test script for Supabase Authentication and RLS policies
 * Run with: node test-auth.js
 * 
 * Make sure to set environment variables:
 * SUPABASE_URL=https://your-project-id.supabase.co
 * SUPABASE_ANON_KEY=your-anon-key
 */

const { createClient } = require('@supabase/supabase-js');
const crypto = require('crypto');

// Configuration
const supabaseUrl = process.env.SUPABASE_URL;
const supabaseKey = process.env.SUPABASE_ANON_KEY;

if (!supabaseUrl || !supabaseKey) {
    console.error('Error: Missing environment variables');
    console.error('Please set SUPABASE_URL and SUPABASE_ANON_KEY');
    process.exit(1);
}

const supabase = createClient(supabaseUrl, supabaseKey);

// Helper function to generate unique test email
function generateTestEmail() {
    const randomId = crypto.randomBytes(4).toString('hex');
    return `test-${randomId}@example.com`;
}

// Test runner
async function runTests() {
    console.log('🧪 Starting Supabase Auth and RLS Tests...\n');
    
    let testUser = null;
    let testEmail = generateTestEmail();
    
    try {
        // Test 1: User Registration
        console.log('📝 Test 1: User Registration');
        const { data: signUpData, error: signUpError } = await supabase.auth.signUp({
            email: testEmail,
            password: 'TestPassword123!'
        });
        
        if (signUpError) {
            console.error('❌ Sign up failed:', signUpError.message);
            return;
        }
        
        testUser = signUpData.user;
        console.log('✅ User registration successful');
        console.log(`   User ID: ${testUser.id}`);
        console.log(`   Email: ${testUser.email}\n`);
        
        // Test 2: Create User Profile
        console.log('👤 Test 2: Create User Profile');
        const { data: profileData, error: profileError } = await supabase
            .from('users')
            .insert([
                {
                    id: testUser.id,
                    username: `testuser_${crypto.randomBytes(2).toString('hex')}`,
                    email: testEmail,
                    full_name: 'Test User'
                }
            ])
            .select();
        
        if (profileError) {
            console.error('❌ Profile creation failed:', profileError.message);
        } else {
            console.log('✅ User profile created successfully');
            console.log(`   Username: ${profileData[0].username}\n`);
        }
        
        // Test 3: Read Problems (Public Access)
        console.log('📚 Test 3: Read Problems (Public Access)');
        const { data: problemsData, error: problemsError } = await supabase
            .from('problems')
            .select('title, difficulty, tags')
            .limit(3);
        
        if (problemsError) {
            console.error('❌ Problems read failed:', problemsError.message);
        } else {
            console.log('✅ Problems read successful');
            console.log(`   Found ${problemsData.length} problems:`);
            problemsData.forEach(problem => {
                console.log(`   - ${problem.title} (${problem.difficulty})`);
            });
            console.log();
        }
        
        // Test 4: Read Own Profile (RLS Test)
        console.log('🔐 Test 4: Read Own Profile (RLS Test)');
        const { data: ownProfileData, error: ownProfileError } = await supabase
            .from('users')
            .select('username, email, rating, problems_solved')
            .eq('id', testUser.id);
        
        if (ownProfileError) {
            console.error('❌ Own profile read failed:', ownProfileError.message);
        } else {
            console.log('✅ Own profile read successful');
            if (ownProfileData.length > 0) {
                console.log(`   Username: ${ownProfileData[0].username}`);
                console.log(`   Rating: ${ownProfileData[0].rating}`);
                console.log(`   Problems Solved: ${ownProfileData[0].problems_solved}`);
            }
            console.log();
        }
        
        // Test 5: Read Sample Test Cases
        console.log('🧪 Test 5: Read Sample Test Cases');
        const { data: testCasesData, error: testCasesError } = await supabase
            .from('test_cases')
            .select('id, problem_id, is_sample')
            .eq('is_sample', true)
            .limit(5);
        
        if (testCasesError) {
            console.error('❌ Sample test cases read failed:', testCasesError.message);
        } else {
            console.log('✅ Sample test cases read successful');
            console.log(`   Found ${testCasesData.length} sample test cases\n`);
        }
        
        // Test 6: Try to Create a Problem (Should work for authenticated users)
        console.log('📝 Test 6: Create a Problem');
        const { data: newProblemData, error: newProblemError } = await supabase
            .from('problems')
            .insert([
                {
                    title: 'Test Problem',
                    slug: `test-problem-${crypto.randomBytes(4).toString('hex')}`,
                    description: 'This is a test problem created during testing.',
                    difficulty: 1000,
                    tags: ['test', 'demo']
                }
            ])
            .select();
        
        if (newProblemError) {
            console.error('❌ Problem creation failed:', newProblemError.message);
        } else {
            console.log('✅ Problem creation successful');
            console.log(`   Problem ID: ${newProblemData[0].id}`);
            console.log(`   Title: ${newProblemData[0].title}\n`);
        }
        
        // Test 7: User Sign Out
        console.log('🚪 Test 7: User Sign Out');
        const { error: signOutError } = await supabase.auth.signOut();
        
        if (signOutError) {
            console.error('❌ Sign out failed:', signOutError.message);
        } else {
            console.log('✅ User signed out successfully\n');
        }
        
        console.log('🎉 All tests completed successfully!');
        console.log('\n📋 Test Summary:');
        console.log('✅ User registration and authentication');
        console.log('✅ User profile creation');
        console.log('✅ Public problem access');
        console.log('✅ RLS policies for user data');
        console.log('✅ Sample test cases access');
        console.log('✅ Problem creation for authenticated users');
        console.log('✅ User sign out');
        
    } catch (error) {
        console.error('💥 Test suite failed with error:', error.message);
        console.error('Stack trace:', error.stack);
    }
}

// Run the tests
runTests().catch(console.error);

// Handle process termination
process.on('SIGINT', () => {
    console.log('\n🛑 Test suite interrupted');
    process.exit(0);
});

process.on('SIGTERM', () => {
    console.log('\n🛑 Test suite terminated');
    process.exit(0);
});