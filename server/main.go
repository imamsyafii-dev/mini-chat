package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	createdirectapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/createdirect"
	currentuserapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/currentuser"
	listconversationsapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/listconversations"
	loginapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/login"
	registerapp "github.com/imamsyafii-dev/mini-chat/server/internal/app/register"
	"github.com/imamsyafii-dev/mini-chat/server/internal/config"
	"github.com/imamsyafii-dev/mini-chat/server/internal/httpapi"
	"github.com/imamsyafii-dev/mini-chat/server/internal/security/password"
	"github.com/imamsyafii-dev/mini-chat/server/internal/security/token"
	postgresstore "github.com/imamsyafii-dev/mini-chat/server/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	serverReadHeaderTimeout = 5 * time.Second
	serverIdleTimeout       = 60 * time.Second
	serverMaxHeaderBytes    = 1 << 20
)

func main() {
	if err := run(); err != nil {
		log.Fatalf(
			"mini chat server: %v",
			err,
		)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf(
			"load config: %w",
			err,
		)
	}

	tokenManager, err := token.NewJWTManager(
		[]byte(cfg.JWTSecret),
		cfg.AccessTokenTTL,
	)
	if err != nil {
		return fmt.Errorf(
			"create JWT manager: %w",
			err,
		)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	database, err := pgxpool.New(
		ctx,
		cfg.PostgresURL,
	)
	if err != nil {
		return fmt.Errorf(
			"open PostgreSQL: %w",
			err,
		)
	}
	defer database.Close()

	if err := database.Ping(ctx); err != nil {
		return fmt.Errorf(
			"ping PostgreSQL: %w",
			err,
		)
	}

	identityStore :=
		postgresstore.NewIdentityStore(
			database,
		)
	conversationStore :=
		postgresstore.NewConversationStore(
			database,
		)
	passwordHasher :=
		password.NewArgon2IDHasher()
	registerUseCase := registerapp.New(
		identityStore,
		passwordHasher,
	)
	loginUseCase := loginapp.New(
		identityStore,
		passwordHasher,
		tokenManager,
	)
	currentUserUseCase := currentuserapp.New(
		identityStore,
	)
	createDirectConversationUseCase :=
		createdirectapp.New(
			identityStore,
			conversationStore,
		)
	listConversationsUseCase :=
		listconversationsapp.New(
			conversationStore,
		)
	handler :=
		httpapi.NewHandlerWithDependencies(
			httpapi.Dependencies{
				Register:                 registerUseCase,
				Login:                    loginUseCase,
				AccessTokenVerifier:      tokenManager,
				CurrentUser:              currentUserUseCase,
				CreateDirectConversation: createDirectConversationUseCase,
				ListConversations:        listConversationsUseCase,
			},
		)

	listener, err := net.Listen(
		"tcp",
		cfg.HTTPAddr,
	)
	if err != nil {
		return fmt.Errorf(
			"listen on %s: %w",
			cfg.HTTPAddr,
			err,
		)
	}
	defer listener.Close()

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		IdleTimeout:       serverIdleTimeout,
		MaxHeaderBytes:    serverMaxHeaderBytes,
	}

	log.Printf(
		"mini chat server listening on http://%s",
		listener.Addr(),
	)

	if err := serve(
		ctx,
		listener,
		server,
		cfg.ShutdownTimeout,
	); err != nil {
		return err
	}

	log.Print(
		"mini chat server stopped",
	)

	return nil
}

func serve(
	ctx context.Context,
	listener net.Listener,
	server *http.Server,
	shutdownTimeout time.Duration,
) error {
	serverErrors := make(
		chan error,
		1,
	)

	go func() {
		serverErrors <- server.Serve(
			listener,
		)
	}()

	select {
	case err := <-serverErrors:
		if err == nil ||
			errors.Is(
				err,
				http.ErrServerClosed,
			) {
			return nil
		}

		return fmt.Errorf(
			"serve HTTP: %w",
			err,
		)

	case <-ctx.Done():
	}

	shutdownCtx, cancel :=
		context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
	defer cancel()

	if err := server.Shutdown(
		shutdownCtx,
	); err != nil {
		closeErr := server.Close()
		if closeErr != nil {
			return fmt.Errorf(
				"shutdown HTTP server: %w; force close: %v",
				err,
				closeErr,
			)
		}

		return fmt.Errorf(
			"shutdown HTTP server: %w",
			err,
		)
	}

	err := <-serverErrors
	if err != nil &&
		!errors.Is(
			err,
			http.ErrServerClosed,
		) {
		return fmt.Errorf(
			"serve HTTP after shutdown: %w",
			err,
		)
	}

	return nil
}
