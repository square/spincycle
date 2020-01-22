/*
This package holds migration files which, when all applied in order,
result in a database schema identical to that described in request_manager_schema.sql. When updating that file, you must add a
new migration file to this directory that reflects the changes made.
Follow the naming convention of the other files in this package
(vXXX_description.sql).

Do not edit a migration file after it has been commited to the master
branch, but create a new migration file to fix any errors.

The test in migration_test.go ensures that the tables produced by
applying all the migrations vs the tables produced by applying request_manager_schema.sql are identical (their CREATE TABLE
statements are identical).
*/
package migrations
