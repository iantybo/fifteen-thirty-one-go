#!/bin/bash

# Configuration: Comment out remotes you don't want to push to
REMOTES=(
    "origin"
    "azure"
    "bitbucket"
    "gitlab"
    # "self-hosted-gitlab"  # Commented out by default
)

# Base branch for PRs (usually main or master)
BASE_BRANCH="main"

# Color output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Get current branch
CURRENT_BRANCH=$(git branch --show-current)

if [ -z "$CURRENT_BRANCH" ]; then
    echo -e "${RED}Error: Not on any branch${NC}"
    exit 1
fi

echo -e "${BLUE}Current branch: $CURRENT_BRANCH${NC}"
echo -e "${BLUE}Pushing to remotes...${NC}\n"

# Array to store PR URLs
declare -a PR_URLS

# Push to each remote and generate PR URLs
for remote in "${REMOTES[@]}"; do
    echo -e "${GREEN}Pushing to $remote...${NC}"

    if git push -u "$remote" "$CURRENT_BRANCH" 2>&1; then
        echo -e "${GREEN}✓ Successfully pushed to $remote${NC}\n"

        # Generate PR URL based on remote
        REMOTE_URL=$(git remote get-url "$remote")

        # GitHub
        if [[ $REMOTE_URL == *"github.com"* ]]; then
            if [[ $REMOTE_URL == git@* ]]; then
                # SSH format: git@github.com:user/repo.git
                REPO_PATH=$(echo "$REMOTE_URL" | sed -e 's/git@github.com://' -e 's/\.git$//')
            else
                # HTTPS format: https://github.com/user/repo.git
                REPO_PATH=$(echo "$REMOTE_URL" | sed -e 's#https://github.com/##' -e 's/\.git$//')
            fi
            PR_URL="https://github.com/$REPO_PATH/compare/$BASE_BRANCH...$CURRENT_BRANCH"
            PR_URLS+=("$PR_URL")

        # GitLab (cloud)
        elif [[ $REMOTE_URL == *"gitlab.com"* ]]; then
            if [[ $REMOTE_URL == git@* ]]; then
                REPO_PATH=$(echo "$REMOTE_URL" | sed -e 's/git@gitlab.com://' -e 's/\.git$//')
            else
                REPO_PATH=$(echo "$REMOTE_URL" | sed -e 's#https://gitlab.com/##' -e 's/\.git$//')
            fi
            PR_URL="https://gitlab.com/$REPO_PATH/-/merge_requests/new?merge_request[source_branch]=$CURRENT_BRANCH&merge_request[target_branch]=$BASE_BRANCH"
            PR_URLS+=("$PR_URL")

        # Self-hosted GitLab
        elif [[ $REMOTE_URL == *"192.168.50.217"* ]]; then
            # Extract repo path for self-hosted GitLab
            REPO_PATH=$(echo "$REMOTE_URL" | sed -e 's#ssh://git@192.168.50.217:2222/##' -e 's/\.git$//')
            PR_URL="http://192.168.50.217/$REPO_PATH/-/merge_requests/new?merge_request[source_branch]=$CURRENT_BRANCH&merge_request[target_branch]=$BASE_BRANCH"
            PR_URLS+=("$PR_URL")

        # Bitbucket
        elif [[ $REMOTE_URL == *"bitbucket.org"* ]]; then
            if [[ $REMOTE_URL == git@* ]]; then
                REPO_PATH=$(echo "$REMOTE_URL" | sed -e 's/git@bitbucket.org://' -e 's/\.git$//')
            else
                REPO_PATH=$(echo "$REMOTE_URL" | sed -e 's#https://bitbucket.org/##' -e 's/\.git$//')
            fi
            PR_URL="https://bitbucket.org/$REPO_PATH/pull-requests/new?source=$CURRENT_BRANCH&dest=$BASE_BRANCH"
            PR_URLS+=("$PR_URL")

        # Azure DevOps
        elif [[ $REMOTE_URL == *"dev.azure.com"* ]]; then
            # Format: https://username@dev.azure.com/org/project/_git/repo
            AZURE_PATH=$(echo "$REMOTE_URL" | sed -e 's#https://[^@]*@dev.azure.com/##' -e 's#https://dev.azure.com/##')
            ORG=$(echo "$AZURE_PATH" | cut -d'/' -f1)
            PROJECT=$(echo "$AZURE_PATH" | cut -d'/' -f2)
            REPO=$(echo "$AZURE_PATH" | cut -d'/' -f4)
            PR_URL="https://dev.azure.com/$ORG/$PROJECT/_git/$REPO/pullrequestcreate?sourceRef=$CURRENT_BRANCH&targetRef=$BASE_BRANCH"
            PR_URLS+=("$PR_URL")
        fi
    else
        echo -e "${RED}✗ Failed to push to $remote${NC}\n"
    fi
done

# Open all PR URLs in browser
if [ ${#PR_URLS[@]} -gt 0 ]; then
    echo -e "${BLUE}Opening PR links in browser...${NC}\n"
    for url in "${PR_URLS[@]}"; do
        echo -e "${GREEN}Opening: $url${NC}"
        open "$url"
        sleep 0.5  # Small delay to avoid overwhelming the browser
    done
else
    echo -e "${RED}No PR URLs to open${NC}"
fi

echo -e "\n${GREEN}Done!${NC}"
