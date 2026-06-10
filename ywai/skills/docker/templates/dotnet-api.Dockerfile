# syntax=docker/dockerfile:1
# .NET 9 API — hardened multi-stage build.
# ADAPT: SDK/runtime version from global.json / TargetFramework; resolve <digest> per SKILL.md.
# ADAPT: replace MyService.Api with the real project name/path.

FROM mcr.microsoft.com/dotnet/sdk:9.0@sha256:<digest> AS build
WORKDIR /src
# Copy project files first so `dotnet restore` stays cached across code changes.
COPY Directory.Packages.props* nuget.config* ./
COPY src/MyService.Api/MyService.Api.csproj src/MyService.Api/
RUN dotnet restore src/MyService.Api/MyService.Api.csproj
COPY . .
RUN dotnet publish src/MyService.Api/MyService.Api.csproj \
    -c Release -o /app/publish --no-restore /p:UseAppHost=false

# Chiseled: distroless-style Ubuntu, no shell or package manager, runs as non-root "app" user.
FROM mcr.microsoft.com/dotnet/aspnet:9.0-noble-chiseled@sha256:<digest> AS runtime
WORKDIR /app
ENV ASPNETCORE_URLS=http://+:8080
COPY --from=build /app/publish .
# ADAPT: must match the Helm containerPort for this service.
EXPOSE 8080
ENTRYPOINT ["dotnet", "MyService.Api.dll"]
