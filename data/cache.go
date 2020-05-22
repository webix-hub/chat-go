package data

func NewUsersCache(dao *DAO) UsersCache {
	return UsersCache{make(map[int]map[int]struct{}), make(map[int]map[int]struct{}), dao}
}

type UsersCache struct {
	// Users hold map of chats for each userId
	Users map[int]map[int]struct{}
	// Chats hold map of users for each chatId
	Chats map[int]map[int]struct{}

	dao *DAO
}

func (cache *UsersCache) JoinChat(userId, chatId int) {
	c, ok := cache.Users[userId]
	if !ok {
		return
	}
	c[chatId] = struct{}{}

	u, ok := cache.Chats[chatId]
	if !ok {
		return
	}
	u[userId] = struct{}{}
}

func (cache *UsersCache) LeaveChat(userId, chatId int) {
	c, ok := cache.Users[userId]
	if !ok {
		return
	}
	delete(c, chatId)

	u, ok := cache.Chats[chatId]
	if !ok {
		return
	}
	delete(u, userId)
}

func (cache *UsersCache) HasChat(userId, chatId int) bool {
	c, ok := cache.Users[userId]
	if !ok {
		c = cache.fillUsers(userId)
	}

	_, has := c[chatId]
	return has
}

func (cache *UsersCache) GetChats(userId int) []int {
	c, ok := cache.Users[userId]
	if !ok {
		c = cache.fillUsers(userId)
	}

	out := make([]int, 0, len(c))
	for k := range c {
		out = append(out, k)
	}

	return out
}

func (cache *UsersCache) GetUsers(chatId int) []int {
	c, ok := cache.Chats[chatId]
	if !ok {
		c = cache.fillChats(chatId)
	}

	out := make([]int, 0, len(c))
	for k := range c {
		out = append(out, k)
	}

	return out
}

func (cache *UsersCache) fillUsers(userId int) map[int]struct{} {
	userChats, _ := cache.dao.UserChats.ByUser(userId)

	chats := make(map[int]struct{})
	for _, userChat := range userChats {
		chats[userChat.ChatID] = struct{}{}
	}

	cache.Users[userId] = chats

	return chats
}

func (cache *UsersCache) fillChats(chatId int) map[int]struct{} {
	userChats, _ := cache.dao.UserChats.ByChat(chatId)

	users := make(map[int]struct{})
	for _, userChat := range userChats {
		users[userChat.UserID] = struct{}{}
	}

	cache.Chats[chatId] = users

	return users
}
