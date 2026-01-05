# Step 1: Use an official Rust image for building the app
FROM rust:latest as builder

# Install required dependencies
RUN apt-get update && apt-get install -y \
    libssl-dev \
    libc6 \
    && rm -rf /var/lib/apt/lists/*

# Set the working directory
WORKDIR /usr/src/imageblur

# Copy the Cargo.toml and Cargo.lock files first (to cache dependencies)
COPY Cargo.toml Cargo.lock ./

# Copy the source code
COPY src ./src

# Step 2: Build the application
RUN cargo build --release

# Step 3: Use a minimal runtime image with updated glibc (debian:bookworm-slim)
FROM debian:bookworm-slim as runtime

# Install dependencies for running the app (including the updated glibc)
RUN apt-get update && apt-get install -y \
    libssl-dev \
    libc6 \
    && rm -rf /var/lib/apt/lists/*

# Copy the built binary from the builder stage
COPY --from=builder /usr/src/imageblur/target/release/imageblur /usr/local/bin/imageblur

# Expose the application port
EXPOSE 8085

# Run the application
CMD ["imageblur"]
