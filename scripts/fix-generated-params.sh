#!/bin/bash
# Fix duplicate parameter names in generated OpenAPI client code
# This script fixes functions like: func NewXxx(server string, server int)
# to: func NewXxx(server string, serverID int)

set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <file.gen.go>"
    exit 1
fi

FILE="$1"

if [ ! -f "$FILE" ]; then
    echo "Error: File not found: $FILE"
    exit 1
fi

# Use perl for in-place editing with more powerful regex
# Strategy: Replace the second occurrence of "server" in function parameter lists

# For functions with exactly 2 parameters: (server string, server TYPE)
perl -i -pe 's/\(server string, server (int|string)\)/(server string, serverID $1)/g' "$FILE"

# For functions with 3+ parameters: (server string, server TYPE, ...)
# Match the second server parameter followed by a type and comma
perl -i -pe 's/\(server string, server (int|string), /(server string, serverID $1, /g' "$FILE"

# Now fix variable usage inside the function bodies
# The path parameter 'server' (now serverID) is used in StyleParamWithLocation calls
# Pattern: StyleParamWithLocation(..., "server", ..., server) -> StyleParamWithLocation(..., "server", ..., serverID)
perl -i -pe 's/(StyleParamWithLocation\([^)]*"server"[^)]*),\s*server\)/$1, serverID)/g' "$FILE"

# Also fix cases where server is used in fmt.Sprintf for server paths (path parameter)
perl -i -pe 's/(fmt\.Sprintf\("\/servers\/%s", )server\)/$1serverID)/g' "$FILE"

# Fix wrapper function calls that pass server twice - the second should be serverID
# Pattern: NewXxxRequestWithBody(server, server, ...) -> NewXxxRequestWithBody(server, serverID, ...)
perl -i -pe 's/RequestWithBody\(server, server, /RequestWithBody(server, serverID, /g' "$FILE"
perl -i -pe 's/RequestWithBody\(server, server\)/RequestWithBody(server, serverID)/g' "$FILE"

echo "Fixed duplicate parameter names in $FILE"
