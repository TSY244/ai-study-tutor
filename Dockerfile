# Docker image for isolated knowledge extraction via Claude Code
# Build: docker build -t ai-tutor-runner .
FROM node:22-slim

# System deps for yt-dlp, ffmpeg, Python
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 python3-pip python3-venv \
    ffmpeg \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install yt-dlp
RUN pip3 install --break-system-packages yt-dlp

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Work directory inside container
WORKDIR /workspace

# Usage: docker run --rm -e ANTHROPIC_AUTH_TOKEN=... -v $(pwd)/course:/output ai-tutor-runner claude -p "..."
