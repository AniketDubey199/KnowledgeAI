package identity

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryRepository provides a thread-safe, in-memory implementation of Repository for unit and integration testing.
type MemoryRepository struct {
	mu           sync.RWMutex
	users        map[uuid.UUID]*User
	usersByEmail map[string]uuid.UUID

	sessions       map[uuid.UUID]*RefreshSession
	sessionsByHash map[string]uuid.UUID

	organizations map[uuid.UUID]*Organization
	memberships   map[string]*Membership // key: orgID:userID
}

// NewMemoryRepository initializes an empty MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:          make(map[uuid.UUID]*User),
		usersByEmail:   make(map[string]uuid.UUID),
		sessions:       make(map[uuid.UUID]*RefreshSession),
		sessionsByHash: make(map[string]uuid.UUID),
		organizations:  make(map[uuid.UUID]*Organization),
		memberships:    make(map[string]*Membership),
	}
}

func (m *MemoryRepository) CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	normEmail := strings.ToLower(email)
	if _, exists := m.usersByEmail[normEmail]; exists {
		return nil, ErrUserAlreadyExists
	}

	now := time.Now().UTC()
	user := &User{
		ID:           uuid.New(),
		Email:        normEmail,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.users[user.ID] = user
	m.usersByEmail[normEmail] = user.ID
	return user, nil
}

func (m *MemoryRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normEmail := strings.ToLower(email)
	id, exists := m.usersByEmail[normEmail]
	if !exists {
		return nil, ErrUserNotFound
	}

	user := m.users[id]
	return user, nil
}

func (m *MemoryRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (m *MemoryRepository) CreateRefreshSession(ctx context.Context, session *RefreshSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashKey := string(session.TokenHash)
	m.sessions[session.ID] = session
	m.sessionsByHash[hashKey] = session.ID
	return nil
}

func (m *MemoryRepository) GetRefreshSessionByHash(ctx context.Context, tokenHash []byte) (*RefreshSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, session := range m.sessions {
		if bytes.Equal(session.TokenHash, tokenHash) {
			// Return a copy to avoid concurrent mutations
			copied := *session
			return &copied, nil
		}
	}
	return nil, ErrSessionNotFound
}

func (m *MemoryRepository) RotateRefreshSession(ctx context.Context, oldSessionID uuid.UUID, newSession *RefreshSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	old, exists := m.sessions[oldSessionID]
	if !exists {
		return ErrSessionNotFound
	}

	now := time.Now().UTC()
	old.RevokedAt = &now
	old.ReplacedBy = &newSession.ID

	hashKey := string(newSession.TokenHash)
	m.sessions[newSession.ID] = newSession
	m.sessionsByHash[hashKey] = newSession.ID
	return nil
}

func (m *MemoryRepository) RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	for _, session := range m.sessions {
		if session.FamilyID == familyID && session.RevokedAt == nil {
			session.RevokedAt = &now
		}
	}
	return nil
}

func (m *MemoryRepository) RevokeRefreshSessionByHash(ctx context.Context, tokenHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	for _, session := range m.sessions {
		if bytes.Equal(session.TokenHash, tokenHash) && session.RevokedAt == nil {
			session.RevokedAt = &now
		}
	}
	return nil
}

func (m *MemoryRepository) CreateOrganization(ctx context.Context, name string, creatorID uuid.UUID) (*Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	org := &Organization{
		ID:        uuid.New(),
		Name:      name,
		CreatedBy: creatorID,
		CreatedAt: now,
	}
	m.organizations[org.ID] = org

	mem := &Membership{
		OrganizationID: org.ID,
		UserID:         creatorID,
		Role:           RoleOwner,
		CreatedAt:      now,
	}
	key := org.ID.String() + ":" + creatorID.String()
	m.memberships[key] = mem

	return org, nil
}

func (m *MemoryRepository) GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (*Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	org, exists := m.organizations[orgID]
	if !exists {
		return nil, ErrOrgNotFound
	}
	return org, nil
}

func (m *MemoryRepository) ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]*UserOrganization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*UserOrganization
	for _, mem := range m.memberships {
		if mem.UserID == userID {
			if org, exists := m.organizations[mem.OrganizationID]; exists {
				result = append(result, &UserOrganization{
					ID:        org.ID,
					Name:      org.Name,
					Role:      mem.Role,
					CreatedAt: org.CreatedAt,
				})
			}
		}
	}
	return result, nil
}

func (m *MemoryRepository) GetMembership(ctx context.Context, orgID, userID uuid.UUID) (*Membership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := orgID.String() + ":" + userID.String()
	mem, exists := m.memberships[key]
	if !exists {
		return nil, ErrMembershipNotFound
	}
	copied := *mem
	return &copied, nil
}

func (m *MemoryRepository) AddMember(ctx context.Context, orgID, userID uuid.UUID, role string) (*Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, orgExists := m.organizations[orgID]; !orgExists {
		return nil, ErrOrgNotFound
	}
	if _, userExists := m.users[userID]; !userExists {
		return nil, ErrUserNotFound
	}

	key := orgID.String() + ":" + userID.String()
	if _, exists := m.memberships[key]; exists {
		return nil, ErrMembershipExists
	}

	now := time.Now().UTC()
	mem := &Membership{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
		CreatedAt:      now,
	}
	m.memberships[key] = mem
	return mem, nil
}

func (m *MemoryRepository) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*MemberDetail, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, orgExists := m.organizations[orgID]; !orgExists {
		return nil, ErrOrgNotFound
	}

	var members []*MemberDetail
	for _, mem := range m.memberships {
		if mem.OrganizationID == orgID {
			if user, exists := m.users[mem.UserID]; exists {
				members = append(members, &MemberDetail{
					UserID:      user.ID,
					Email:       user.Email,
					DisplayName: user.DisplayName,
					Role:        mem.Role,
					JoinedAt:    mem.CreatedAt,
				})
			}
		}
	}
	return members, nil
}

func (m *MemoryRepository) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := orgID.String() + ":" + userID.String()
	if _, exists := m.memberships[key]; !exists {
		return ErrMembershipNotFound
	}
	delete(m.memberships, key)
	return nil
}

func (m *MemoryRepository) CountOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, mem := range m.memberships {
		if mem.OrganizationID == orgID && mem.Role == RoleOwner {
			count++
		}
	}
	return count, nil
}
