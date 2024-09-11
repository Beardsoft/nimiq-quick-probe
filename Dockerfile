# Start from the official Golang image
FROM golang:1.22-alpine

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code from the current directory to the Working Directory inside the container
COPY ./cmd ./cmd

# Build the Go app, specifying the location of the main.go file
RUN go build -o /health-probe ./cmd

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["/health-probe"]
